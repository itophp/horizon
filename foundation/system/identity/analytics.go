package identity

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"time"

	"horizon/foundation/kernel/capability/contracts"
)

const (
	analyticsCapability    = "identity.analytics.read"
	analyticsSourceVersion = "1.0.0"
)

// AnalyticsRuntimeSource identifies the deployment package for the workspace fallback.
const AnalyticsRuntimeSource = "builtin:analytics"

type analyticsInvoke func(context.Context, string, string, contracts.Input) (*contracts.Result, error)

// AttachAnalyticsGateway connects the read composition to the application Gateway.
func (s *Service) AttachAnalyticsGateway(invoke analyticsInvoke) {
	s.analyticsInvoke = invoke
}

func analyticsDefinition() contracts.Definition {
	number := &contracts.Schema{Type: "integer"}
	str := &contracts.Schema{Type: "string"}
	series := func(key string, limit int) *contracts.Schema {
		return contracts.Array(contracts.Object(map[string]*contracts.Schema{
			key: str, "value": number,
		}, key, "value"), 0, limit)
	}
	d := readDefinition(analyticsCapability, "Read identity statistics",
		"Read bounded statistics for identity records visible to the signed-in account.",
		"account", "", contracts.Object(map[string]*contracts.Schema{
			"accounts": contracts.Object(map[string]*contracts.Schema{"active": number, "total": number}, "active", "total"),
			"organizations": contracts.Object(map[string]*contracts.Schema{
				"total": number, "enterprises": number, "companies": number,
			}, "total", "enterprises", "companies"),
			"roles": number, "departments": number,
			"trend": series("date", 7), "distribution": series("label", 4),
			"recentAccounts": contracts.Array(contracts.Object(map[string]*contracts.Schema{
				"id": number, "fullName": str, "username": str, "status": str, "createdAt": str,
				"departments": &contracts.Schema{Type: "array", Items: str},
			}, "id", "fullName", "username", "status", "createdAt", "departments"), 0, 5),
			"activity": contracts.Array(contracts.Object(map[string]*contracts.Schema{
				"key": str, "id": number, "kind": contracts.Enum("account", "organization", "department", "role"),
				"name": str, "code": str, "createdAt": str,
			}, "key", "id", "kind", "name", "code", "createdAt"), 0, 10),
		}, "activity"))
	d.Layer = contracts.LayerOrchestration
	d.Reads = []string{"account", "organization", "department", "role"}
	d.AllowedOrigins = []contracts.Origin{contracts.OriginHumanUI}
	d.Tags = append(d.Tags, "manual-only", contracts.AuthenticatedProviderTag)
	d.Input = contracts.Object(map[string]*contracts.Schema{"timeZone": contracts.Str(1, 100)})
	return d
}

var analyticsSources = []struct {
	kind, permission, capability string
}{
	{"account", "accounts:read", "identity.accounts.admin.read"},
	{"organization", "organizations:read", "identity.organizations.read"},
	{"department", "departments:read", "identity.departments.read"},
	{"role", "roles:read", "identity.roles.read"},
}

func (s *Service) analyticsRead(ctx context.Context, input contracts.Input) (contracts.Output, error) {
	principal, err := Current(ctx)
	if err != nil {
		return contracts.Output{}, err
	}
	if principal.Account.ID == 0 {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	zone, _ := input["timeZone"].(string)
	if zone == "" {
		zone = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(zone)
	if err != nil || zone == "Local" {
		return contracts.Output{}, contracts.Problem("invalid")
	}

	sections := make(map[string][]map[string]any, len(analyticsSources))
	for _, source := range analyticsSources {
		if !principal.Can(source.permission) {
			continue
		}
		if s.analyticsInvoke == nil {
			return contracts.Output{}, contracts.Problem("unavailable")
		}
		result, err := s.analyticsInvoke(ctx, source.capability, analyticsSourceVersion, contracts.Input{})
		if err != nil {
			return contracts.Output{}, err
		}
		if result.Status != contracts.StatusSucceeded {
			return contracts.Output{}, contracts.Problem("unavailable")
		}
		payload, ok := result.Data.(map[string]any)
		items, itemsOK := payload["items"].([]any)
		if !ok || !itemsOK {
			return contracts.Output{}, contracts.Problem("internal")
		}
		sections[source.kind] = analyticsRecords(items)
	}
	return contracts.Output{Data: analyticsProjection(sections, time.Now().In(location))}, nil
}

func analyticsRecords(value any) []map[string]any {
	values, _ := value.([]any)
	records := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if record, ok := value.(map[string]any); ok {
			records = append(records, record)
		}
	}
	return records
}

type analyticsActivity struct {
	record map[string]any
	date   time.Time
	kind   string
	id     float64
}

func analyticsProjection(sections map[string][]map[string]any, today time.Time) map[string]any {
	out := map[string]any{}
	accounts, canReadAccounts := sections["account"]
	departments, canReadDepartments := sections["department"]
	activity := []analyticsActivity{}
	for _, source := range analyticsSources {
		for _, record := range sections[source.kind] {
			date, _ := time.Parse(time.RFC3339Nano, analyticsString(record["createdAt"]))
			id, _ := record["id"].(float64)
			activity = append(activity, analyticsActivity{record, date, source.kind, id})
		}
	}
	slices.SortFunc(activity, func(a, b analyticsActivity) int {
		if order := b.date.Compare(a.date); order != 0 {
			return order
		}
		if order := strings.Compare(a.kind, b.kind); order != 0 {
			return order
		}
		if a.id < b.id {
			return -1
		}
		if a.id > b.id {
			return 1
		}
		return 0
	})
	recent := make([]map[string]any, 0, 10)
	for _, item := range activity[:min(10, len(activity))] {
		name, code := item.record["name"], item.record["code"]
		if item.kind == "account" {
			name, code = item.record["fullName"], item.record["username"]
		}
		recent = append(recent, map[string]any{
			"key": item.kind + "-" + strconv.FormatFloat(item.id, 'f', 0, 64), "kind": item.kind,
			"id": item.id, "name": name, "code": code, "createdAt": item.record["createdAt"],
		})
	}
	out["activity"] = recent
	if organizations, allowed := sections["organization"]; allowed {
		enterprises, companies := 0, 0
		for _, organization := range organizations {
			switch organization["kind"] {
			case "enterprise":
				enterprises++
			case "company":
				companies++
			}
		}
		out["organizations"] = map[string]any{"total": len(organizations), "enterprises": enterprises, "companies": companies}
	}
	if roles, allowed := sections["role"]; allowed {
		out["roles"] = len(roles)
	}
	if canReadDepartments {
		out["departments"] = len(departments)
	}
	if canReadAccounts {
		addAccountAnalytics(out, accounts, activity, today)
	}
	if canReadAccounts && canReadDepartments {
		out["distribution"] = departmentDistribution(accounts, departments)
	}
	return out
}

func addAccountAnalytics(out map[string]any, accounts []map[string]any, activity []analyticsActivity, today time.Time) {
	active := 0
	for _, account := range accounts {
		if account["status"] == "active" {
			active++
		}
	}
	out["accounts"] = map[string]any{"active": active, "total": len(accounts)}
	trend := make([]map[string]any, 7)
	day := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location()).AddDate(0, 0, -6)
	for index := range trend {
		next := day.AddDate(0, 0, 1)
		count := 0
		for _, item := range activity {
			if item.kind == "account" && !item.date.IsZero() && item.date.Before(next) {
				count++
			}
		}
		trend[index] = map[string]any{"date": day.Format(time.DateOnly), "value": count}
		day = next
	}
	out["trend"] = trend
	recent := make([]map[string]any, 0, 5)
	for _, item := range activity {
		if item.kind != "account" {
			continue
		}
		row := projectFields(item.record, []string{"id", "fullName", "username", "status", "createdAt"}).(map[string]any)
		names := []string{}
		for _, membership := range analyticsRecords(item.record["memberships"]) {
			if name := analyticsString(membership["departmentName"]); name != "" {
				names = append(names, name)
			}
		}
		row["departments"] = names
		recent = append(recent, row)
		if len(recent) == 5 {
			break
		}
	}
	out["recentAccounts"] = recent
}

func departmentDistribution(accounts, departments []map[string]any) []map[string]any {
	counts := map[float64]int{}
	for _, account := range accounts {
		seen := map[float64]bool{}
		for _, membership := range analyticsRecords(account["memberships"]) {
			if id, ok := membership["departmentId"].(float64); ok && !seen[id] {
				seen[id] = true
				counts[id]++
			}
		}
	}
	rows := make([]map[string]any, 0, 4)
	for _, department := range departments[:min(4, len(departments))] {
		id, _ := department["id"].(float64)
		rows = append(rows, map[string]any{"label": department["name"], "value": counts[id]})
	}
	return rows
}

func analyticsString(value any) string {
	result, _ := value.(string)
	return result
}
