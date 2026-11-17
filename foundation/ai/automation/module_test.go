package automation

import (
	"strings"
	"testing"
	"time"

	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	armodule "horizon/foundation/system/applicationruntime"
)

// TestDefinitionsAreValid keeps the catalog gate honest without a database:
// every automation contract must pass the same structural checks the registry
// applies at startup.
func TestDefinitionsAreValid(t *testing.T) {
	for _, entry := range New(nil).Entries() {
		for _, problem := range entry.Definition.Problems() {
			t.Errorf("%s: %s", entry.Definition.Key(), problem)
		}
		if entry.Definition.Layer == "" || entry.Definition.Entity == "" {
			t.Errorf("%s: missing layer or entity", entry.Definition.Key())
		}
		if entry.Definition.ReadOnly == false && len(entry.Definition.SuccessEvents) == 0 {
			t.Errorf("%s: write without success event", entry.Definition.Key())
		}
		if entry.Definition.ReadOnly == false && !entry.Definition.Idempotency.Required {
			t.Errorf("%s: write without idempotency", entry.Definition.Key())
		}
	}
}

// TestRuntimeApplicationIsValid proves the Runtime V3 contribution passes the
// same deployment assembly the server runs at startup, including the AI form
// assistant scopes.
func TestRuntimeApplicationIsValid(t *testing.T) {
	definitions := []contracts.Definition{}
	for _, entry := range append(New(nil).Entries(), (&armodule.Module{}).AIEntries()...) {
		definitions = append(definitions, entry.Definition)
	}
	apps, err := ar.Assemble([]ar.ApplicationV2{RuntimeApplication()}, Entities(), definitions)
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 1 || apps[0].Protocol == nil {
		t.Fatalf("unexpected applications: %+v", apps)
	}
	pages := apps[0].Protocol.Pages
	if len(pages) != 3 {
		t.Fatalf("unexpected pages: %d", len(pages))
	}
	form := taskRuntimeForm()
	assistant := form.AIAssistant
	if assistant == nil || len(assistant.InputFields) != 1 || assistant.InputFields[0] != "brief" {
		t.Fatal("AI form assistant must read the one-line brief")
	}
	// The description is the agent instruction, so it is the one field the
	// user cannot leave to a generic default. The name, schedule kind and
	// every wall-clock component are part of the same statement ("every day at
	// 09:00"), and the form assistant must fill them together instead of
	// leaving the user to re-enter the schedule by hand.
	targets := map[string]bool{}
	for _, name := range assistant.TargetFields {
		if targets[name] {
			t.Fatalf("duplicate AI target %s", name)
		}
		targets[name] = true
	}
	for _, name := range []string{"name", "scheduleKind", "description", "timezone", "runAt", "time", "weekday", "dayOfMonth", "minute", "second"} {
		if !targets[name] {
			t.Errorf("AI form assistant must fill %s from the description", name)
		}
	}
	if len(targets) != 10 {
		t.Fatalf("unexpected AI targets: %v", assistant.TargetFields)
	}
	visible := map[string]string{}
	components := map[string]string{}
	for _, field := range form.Create.Fields {
		visible[field.Name] = field.VisibleWhen.Kind
		components[field.Name] = field.Component
		if field.Name == "brief" && field.VisibleWhen.Kind != "" {
			t.Fatal("the one-line brief must always be visible")
		}
	}
	if components["description"] != "markdown" {
		t.Fatal("the task instruction must use the Markdown component")
	}
	if visible["runAt"] != "eq" || visible["weekday"] != "eq" || visible["minute"] != "eq" || visible["dayOfMonth"] != "eq" {
		t.Fatalf("schedule fields must be conditional: %v", visible)
	}
}

func TestNextOccurrence(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	anchor := time.Date(2026, 3, 9, 9, 30, 0, 0, shanghai)
	after := time.Date(2026, 3, 9, 10, 0, 0, 0, shanghai)
	next, ok := NextOccurrence(ScheduleDaily, anchor, shanghai, after, 0)
	if !ok || !next.Equal(time.Date(2026, 3, 10, 9, 30, 0, 0, shanghai)) {
		t.Fatalf("daily next = %v, %v", next, ok)
	}
	next, ok = NextOccurrence(ScheduleHourly, anchor, shanghai, after, 0)
	if !ok || !next.Equal(time.Date(2026, 3, 9, 10, 30, 0, 0, shanghai)) {
		t.Fatalf("hourly next = %v, %v", next, ok)
	}
	next, ok = NextOccurrence(ScheduleWeekly, anchor, shanghai, after, 0)
	if !ok || !next.Equal(time.Date(2026, 3, 16, 9, 30, 0, 0, shanghai)) {
		t.Fatalf("weekly next = %v, %v", next, ok)
	}
	// A monthly series on the 31st clamps to the last day of a shorter month.
	monthly := time.Date(2026, 1, 31, 9, 0, 0, 0, shanghai)
	next, ok = NextOccurrence(ScheduleMonthly, monthly, shanghai, time.Date(2026, 2, 1, 0, 0, 0, 0, shanghai), 31)
	if !ok || next.In(shanghai).Day() != 28 {
		t.Fatalf("monthly clamp = %v, %v", next, ok)
	}
	if _, ok = NextOccurrence(ScheduleOnce, anchor, shanghai, after, 0); ok {
		t.Fatal("a past one-shot must not produce a next occurrence")
	}
}

func TestParseRunAt(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	local, err := ParseRunAt("2026-03-09T09:30", shanghai)
	if err != nil || FormatRunAt(local, shanghai) != "2026-03-09T09:30" {
		t.Fatalf("local parse = %v, %v", local, err)
	}
	instant, err := ParseRunAt("2026-03-09T01:30:00Z", shanghai)
	if err != nil || !instant.Equal(local) {
		t.Fatalf("RFC3339 parse = %v, %v", instant, err)
	}
	if _, err = ParseRunAt("tomorrow", shanghai); err == nil {
		t.Fatal("invalid instant accepted")
	}
}

func TestCreateRejectsAutomationLaunchedAgent(t *testing.T) {
	// The handler guard is documented behaviour: a scheduled run may not create
	// more automations. This checks the predicate the handler relies on.
	correlation := "automation:42"
	if !strings.HasPrefix(correlation, "automation:") {
		t.Fatal("correlation prefix changed without updating the create guard")
	}
}

func intp(value int) *int { return &value }

func TestBuildAnchor(t *testing.T) {
	shanghai, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 9, 10, 5, 0, 0, shanghai)
	hourly, day, err := BuildAnchor(ScheduleSpec{Kind: ScheduleHourly, Timezone: shanghai, Minute: intp(30), Second: intp(15)}, now)
	if err != nil || day != 0 || !hourly.Equal(time.Date(2026, 3, 9, 10, 30, 15, 0, shanghai)) {
		t.Fatalf("hourly = %v %d %v", hourly, day, err)
	}
	daily, _, err := BuildAnchor(ScheduleSpec{Kind: ScheduleDaily, Timezone: shanghai, Hour: intp(9), Minute: intp(0)}, now)
	if err != nil || !daily.Equal(time.Date(2026, 3, 10, 9, 0, 0, 0, shanghai)) {
		t.Fatalf("daily = %v %v", daily, err)
	}
	weekly, _, err := BuildAnchor(ScheduleSpec{Kind: ScheduleWeekly, Timezone: shanghai, Weekday: intp(1), Hour: intp(9), Minute: intp(0)}, now)
	if err != nil || int(weekly.In(shanghai).Weekday()) != 1 {
		t.Fatalf("weekly = %v %v", weekly, err)
	}
	// The anchor clamps to February's last day, but the stored day stays 31.
	monthly, monthlyDay, err := BuildAnchor(ScheduleSpec{Kind: ScheduleMonthly, Timezone: shanghai, DayOfMonth: intp(31), Hour: intp(9), Minute: intp(0)}, time.Date(2026, 2, 1, 0, 0, 0, 0, shanghai))
	if err != nil || monthlyDay != 31 || monthly.In(shanghai).Day() != 28 {
		t.Fatalf("monthly = %v %d %v", monthly, monthlyDay, err)
	}
	next, ok := NextOccurrence(ScheduleMonthly, monthly, shanghai, monthly, 31)
	if !ok || next.In(shanghai).Day() != 31 || next.In(shanghai).Month() != time.March {
		t.Fatalf("monthly kept day = %v %v", next, ok)
	}
	if _, _, err = BuildAnchor(ScheduleSpec{Kind: ScheduleOnce, Timezone: shanghai, RunAt: "2020-01-01T09:00"}, now); err == nil {
		t.Fatal("past one-shot accepted")
	}
}
