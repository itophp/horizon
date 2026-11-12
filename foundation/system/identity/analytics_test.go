package identity

import (
	"testing"
	"time"

	"horizon/foundation/kernel/capability/contracts"
)

func TestAnalyticsProjectionCalendarAndMemberships(t *testing.T) {
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	account := func(id float64, createdAt string, memberships ...any) map[string]any {
		return map[string]any{"id": id, "fullName": "Account", "username": "account",
			"createdAt": createdAt, "status": "active", "email": "private@example.test",
			"memberships": memberships}
	}
	member := func(id float64) map[string]any {
		return map[string]any{"departmentId": id, "departmentName": "Team"}
	}
	sections := map[string][]map[string]any{
		"account": {
			account(8, "2026-03-09T03:59:59Z", member(1), member(1), member(2)),
			account(5, "2026-03-09T04:00:00Z", member(2)),
			account(3, "2026-03-09T04:00:00Z"),
		},
		"department": {
			{"id": float64(1), "name": "First", "code": "FIRST", "createdAt": "2026-01-01T00:00:00Z"},
			{"id": float64(2), "name": "Second", "code": "SECOND", "createdAt": "2026-01-01T00:00:00Z"},
		},
	}
	got := analyticsProjection(sections, time.Date(2026, 3, 9, 18, 0, 0, 0, location))
	trend := got["trend"].([]map[string]any)
	if len(trend) != 7 || trend[5]["date"] != "2026-03-08" || trend[5]["value"] != 1 || trend[6]["value"] != 3 {
		t.Fatal(trend)
	}
	bars := got["distribution"].([]map[string]any)
	if bars[0]["value"] != 1 || bars[1]["value"] != 2 {
		t.Fatal(bars)
	}
	rows := got["recentAccounts"].([]map[string]any)
	if rows[0]["id"] != float64(3) || rows[1]["id"] != float64(5) || rows[2]["id"] != float64(8) {
		t.Fatal(rows)
	}
	if _, ok := rows[0]["email"]; ok {
		t.Fatal("email leaked into the dashboard projection")
	}
	if err := analyticsDefinition().Output.Validate(contracts.Input(camelData(got).(map[string]any))); err != nil {
		t.Fatal(err)
	}
}
