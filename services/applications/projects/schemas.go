package projects

import (
	"strconv"
	"strings"
	"time"

	"baseline/services/api/kernel/capability/contracts"

	"github.com/jackc/pgx/v5/pgtype"
)

// Closed value sets. They mirror the CHECK constraints one to one, so a change
// here without a migration is caught by check-entities.
var (
	priorities    = []string{"low", "normal", "high", "urgent"}
	projectStates = []string{"active", "paused", "completed", "archived"}
	planStates    = []string{"pending", "in_progress", "done", "cancelled"}
	planTypes     = []string{"milestone", "phase", "deliverable"}
	taskStates    = []string{"todo", "in_progress", "done", "cancelled"}
)

func projectProperties() map[string]*contracts.Schema {
	return map[string]*contracts.Schema{
		"id": {Type: "integer"}, "code": {Type: "string"}, "name": {Type: "string"},
		"status": {Type: "string"}, "priority": {Type: "string"},
		"dueDate": {Type: "string", Description: "Empty when the project has no planned end date."},
		"ownerId": {Type: "integer"}, "ownerName": {Type: "string"},
		"planCount": {Type: "integer"}, "taskCount": {Type: "integer"},
		"taskDoneCount": {Type: "integer"}, "taskOverdueCount": {Type: "integer"},
		"createdAt": {Type: "string"}, "updatedAt": {Type: "string"},
	}
}

func planProperties() map[string]*contracts.Schema {
	return map[string]*contracts.Schema{
		"id": {Type: "integer"}, "projectId": {Type: "integer"},
		"planType": {Type: "string"}, "title": {Type: "string"},
		"content": {Type: "string", Description: "Markdown body."},
		"status":  {Type: "string"}, "priority": {Type: "string"},
		"dueDate": {Type: "string"}, "ownerId": {Type: "integer"}, "ownerName": {Type: "string"},
		"createdAt": {Type: "string"}, "updatedAt": {Type: "string"},
	}
}

func taskProperties() map[string]*contracts.Schema {
	return map[string]*contracts.Schema{
		"id": {Type: "integer"}, "projectId": {Type: "integer"},
		"planId":    {Type: "integer", Description: "Zero when the task is not inside a plan."},
		"planTitle": {Type: "string"}, "title": {Type: "string"},
		"description": {Type: "string", Description: "Markdown description."},
		"status":      {Type: "string"}, "priority": {Type: "string"},
		"assigneeId":   {Type: "integer", Description: "Zero when the task is unassigned."},
		"assigneeName": {Type: "string"}, "dueDate": {Type: "string"},
		"completedAt": {Type: "string"},
		"createdAt":   {Type: "string"}, "updatedAt": {Type: "string"},
	}
}

// integer reads an id or counter. The gateway already validated the type; the
// extra cases keep a JSON decoder difference from becoming a silent zero.
func integer(in contracts.Input, key string) int64 {
	switch value := in[key].(type) {
	case float64:
		return int64(value)
	case int64:
		return value
	case int:
		return int64(value)
	case string:
		parsed, _ := strconv.ParseInt(value, 10, 64)
		return parsed
	}
	return 0
}

func text(in contracts.Input, key string) string {
	value, _ := in[key].(string)
	return strings.TrimSpace(value)
}

// nullableID turns zero into NULL, which is how a caller clears a plan or an
// assignee reference.
func nullableID(in contracts.Input, key string) pgtype.Int8 {
	id := integer(in, key)
	if id <= 0 {
		return pgtype.Int8{}
	}
	return pgtype.Int8{Int64: id, Valid: true}
}

// dateValue parses the YYYY-MM-DD contract. An empty value clears the date.
func dateValue(in contracts.Input, key string) (pgtype.Date, error) {
	raw := text(in, key)
	if raw == "" {
		return pgtype.Date{}, nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return pgtype.Date{}, contracts.Problem("invalid")
	}
	return pgtype.Date{Time: parsed, Valid: true}, nil
}

func dateText(value pgtype.Date) string {
	if !value.Valid {
		return ""
	}
	return value.Time.Format("2006-01-02")
}

func timeText(value pgtype.Timestamptz) string {
	if !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339)
}

func idText(value pgtype.Int8) int64 {
	if !value.Valid {
		return 0
	}
	return value.Int64
}

func optText(value pgtype.Text) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

// boundedLimit keeps a list call from selecting the whole table. It mirrors the
// default used by example.record.list.
func boundedLimit(in contracts.Input, fallback, max int32) int32 {
	value := int32(integer(in, "limit"))
	if value < 1 {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}

func allowed(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func paging(in contracts.Input, fallback int32) (int32, int32) {
	page, size := int32(integer(in, "page")), int32(integer(in, "pageSize"))
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = boundedLimit(in, fallback, 100)
	}
	return page, size
}
