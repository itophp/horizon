// Package harness runs Eino agents against the capability gateway. It owns the
// AI task record (an independent lifecycle from the business invocation) and
// joins each task with the capability calls it caused.
package harness

import (
	"encoding/json"
	"time"

	"baseline/services/api/internal/platform/audit"
)

// Run status values.
const (
	StatusRunning          = "running"
	StatusCompleted        = "completed"
	StatusAwaitingApproval = "awaiting_approval"
	StatusFailed           = "failed"
	StatusCancelled        = "cancelled"
)

// Conversation groups the caller's tasks by session. The workbench shows it as
// the task list, so a finished thread can be reopened and continued. Title is
// the owner's rename when present, otherwise the first question; Archived keeps
// the entry out of the active list without deleting the thread.
type Conversation struct {
	SessionID    string
	Title        string
	MessageCount int
	UpdatedAt    time.Time
	Archived     bool
}

// Run is one AI task: either a user conversation or an event-triggered job.
type Run struct {
	ID                 int64
	TenantID           string
	AccountID          int64
	DepartmentID       int64
	SessionID          string
	Agent              string
	Model              string
	InstructionVersion string
	Question           string
	Answer             string
	Status             string
	ErrorCode          string
	ToolCalls          int
	PromptTokens       int
	CompletionTokens   int
	Duration           time.Duration
	TraceID            string
	Origin             string
	CorrelationID      string
	CausationID        string
	TriggerDepth       int
	TriggerPolicy      string
	EventID            string
	Context            json.RawMessage
	CreatedAt          time.Time
	FinishedAt         time.Time
	// Invocations is filled by detail reads so the UI can render the business
	// calls the task made without a second, divergent step table.
	Invocations []audit.Invocation
}

// Result is the terminal projection written to the run.
type Result struct {
	Status           string
	Answer           string
	ErrorCode        string
	ToolCalls        int
	PromptTokens     int
	CompletionTokens int
	Duration         time.Duration
}
