package harness

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"baseline/services/api/internal/platform/audit"
	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/repository/dbgen"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store owns the AI task record. Business calls live in the invocation audit; a
// task is joined to them by correlation id.
type Store struct{ pool *pgxpool.Pool }

// NewStore builds the store. A nil pool keeps the API bootable without a
// database while every agent operation reports unavailable.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Available reports whether state can be written.
func (s *Store) Available() bool { return s != nil && s.pool != nil }

// Start writes the task header before the model is called.
func (s *Store) Start(ctx context.Context, run Run) (*Run, error) {
	if !s.Available() {
		return nil, contracts.Problem("unavailable")
	}
	contextDocument := run.Context
	if len(contextDocument) == 0 {
		contextDocument = json.RawMessage(`{}`)
	}
	row, err := dbgen.New(s.pool).AICreateRun(ctx, dbgen.AICreateRunParams{
		TenantID: run.TenantID, AccountID: run.AccountID, DepartmentID: run.DepartmentID,
		SessionID: run.SessionID, Agent: run.Agent, Model: run.Model,
		InstructionVersion: run.InstructionVersion, Question: run.Question,
		TraceID: run.TraceID, Context: contextDocument, Origin: run.Origin,
		CorrelationID: run.CorrelationID, CausationID: run.CausationID,
		TriggerDepth: int32(run.TriggerDepth), TriggerPolicy: run.TriggerPolicy, EventID: run.EventID,
	})
	if err != nil {
		return nil, err
	}
	run.ID = row.ID
	run.CreatedAt = row.CreatedAt.Time
	run.Status = StatusRunning
	run.Context = contextDocument
	return &run, nil
}

// Finish closes a task.
func (s *Store) Finish(ctx context.Context, tenant string, id int64, result Result) error {
	if !s.Available() {
		return contracts.Problem("unavailable")
	}
	return dbgen.New(s.pool).AIFinishRun(ctx, dbgen.AIFinishRunParams{
		TenantID: tenant, ID: id, Status: result.Status, Answer: result.Answer,
		ErrorCode: result.ErrorCode, ToolCalls: int32(result.ToolCalls),
		PromptTokens: int32(result.PromptTokens), CompletionTokens: int32(result.CompletionTokens),
		DurationMs: int32(result.Duration.Milliseconds()),
	})
}

// List returns the caller's own tasks, newest first.
func (s *Store) List(ctx context.Context, tenant string, accountID int64, limit int) ([]Run, error) {
	if !s.Available() {
		return nil, contracts.Problem("unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := dbgen.New(s.pool).AIListRuns(ctx, dbgen.AIListRunsParams{TenantID: tenant, AccountID: accountID, MaxRows: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]Run, 0, len(rows))
	for _, row := range rows {
		out = append(out, runFromListRow(row))
	}
	return out, nil
}

// ListConversations groups the caller's tasks by session, newest first.
func (s *Store) ListConversations(ctx context.Context, tenant string, accountID int64, limit int) ([]Conversation, error) {
	if !s.Available() {
		return nil, contracts.Problem("unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := dbgen.New(s.pool).AIListConversations(ctx, dbgen.AIListConversationsParams{
		TenantID: tenant, AccountID: accountID, MaxRows: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Conversation, 0, len(rows))
	for _, row := range rows {
		out = append(out, conversationFromListRow(row))
	}
	return out, nil
}

// Conversation returns one of the caller's conversations. The caller's own runs
// are the authority: a session id that belongs to another account or tenant is
// reported as missing rather than updated.
func (s *Store) Conversation(ctx context.Context, tenant string, accountID int64, sessionID string) (Conversation, error) {
	if !s.Available() {
		return Conversation{}, contracts.Problem("unavailable")
	}
	row, err := dbgen.New(s.pool).AIGetConversation(ctx, dbgen.AIGetConversationParams{
		TenantID: tenant, AccountID: accountID, SessionID: sessionID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return Conversation{}, contracts.Problem("not_found")
	}
	if err != nil {
		return Conversation{}, err
	}
	return Conversation{
		SessionID: row.SessionID, Title: row.Title,
		MessageCount: int(row.MessageCount), UpdatedAt: row.UpdatedAt.Time, Archived: row.Archived,
	}, nil
}

// Rename stores the owner's title for a conversation. The first run keeps
// working when the row is absent: the title is derived again.
func (s *Store) Rename(ctx context.Context, tenant string, accountID int64, sessionID, title string) error {
	if !s.Available() {
		return contracts.Problem("unavailable")
	}
	return dbgen.New(s.pool).AIRenameConversation(ctx, dbgen.AIRenameConversationParams{
		TenantID: tenant, AccountID: accountID, SessionID: sessionID, Title: title,
	})
}

// SetArchived marks a conversation archived or restores it.
func (s *Store) SetArchived(ctx context.Context, tenant string, accountID int64, sessionID string, archived bool) error {
	if !s.Available() {
		return contracts.Problem("unavailable")
	}
	return dbgen.New(s.pool).AISetConversationArchived(ctx, dbgen.AISetConversationArchivedParams{
		TenantID: tenant, AccountID: accountID, SessionID: sessionID, Archived: archived,
	})
}

// ListSession returns the tasks of one conversation in the order they happened.
func (s *Store) ListSession(ctx context.Context, tenant string, accountID int64, sessionID string, limit int) ([]Run, error) {
	if !s.Available() {
		return nil, contracts.Problem("unavailable")
	}
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	rows, err := dbgen.New(s.pool).AIListSessionRuns(ctx, dbgen.AIListSessionRunsParams{
		TenantID: tenant, AccountID: accountID, SessionID: sessionID, MaxRows: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]Run, 0, len(rows))
	for _, row := range rows {
		out = append(out, runFromSessionRow(row))
	}
	return out, nil
}

// Get returns one task owned by the caller.
func (s *Store) Get(ctx context.Context, tenant string, accountID, id int64) (*Run, error) {
	if !s.Available() {
		return nil, contracts.Problem("unavailable")
	}
	row, err := dbgen.New(s.pool).AIGetRun(ctx, dbgen.AIGetRunParams{TenantID: tenant, AccountID: accountID, ID: id})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, contracts.Problem("not_found")
	}
	if err != nil {
		return nil, err
	}
	run := runFromGetRow(row)
	return &run, nil
}

// AttachInvocations joins the task with the business calls it caused.
func AttachInvocations(ctx context.Context, store *audit.Store, run *Run) {
	if store == nil || run == nil {
		return
	}
	correlation := run.CorrelationID
	if correlation == "" {
		correlation = run.TraceID
	}
	if correlation == "" {
		return
	}
	invocations, err := store.ListByCorrelation(ctx, run.TenantID, correlation)
	if err != nil {
		return
	}
	run.Invocations = invocations
}

// Summary returns the tenant level evaluation snapshot.
func (s *Store) Summary(ctx context.Context, tenant string) (dbgen.AIEvaluationRow, error) {
	if !s.Available() {
		return dbgen.AIEvaluationRow{}, contracts.Problem("unavailable")
	}
	return dbgen.New(s.pool).AIEvaluation(ctx, tenant)
}

func conversationFromListRow(row dbgen.AIListConversationsRow) Conversation {
	return Conversation{
		SessionID: row.SessionID, Title: row.Title,
		MessageCount: int(row.MessageCount), UpdatedAt: row.UpdatedAt.Time, Archived: row.Archived,
	}
}

func runFromListRow(row dbgen.AIListRunsRow) Run {
	return Run{
		ID: row.ID, AccountID: row.AccountID, DepartmentID: row.DepartmentID, SessionID: row.SessionID,
		Agent: row.Agent, Model: row.Model, InstructionVersion: row.InstructionVersion,
		Question: row.Question, Answer: row.Answer, Status: row.Status, ErrorCode: row.ErrorCode,
		ToolCalls: int(row.ToolCalls), PromptTokens: int(row.PromptTokens), CompletionTokens: int(row.CompletionTokens),
		Duration: time.Duration(row.DurationMs) * time.Millisecond, TraceID: row.TraceID,
		Origin: row.Origin, CorrelationID: row.CorrelationID, CausationID: row.CausationID,
		TriggerDepth: int(row.TriggerDepth), TriggerPolicy: row.TriggerPolicy, EventID: row.EventID,
		Context: json.RawMessage(row.Context), CreatedAt: row.CreatedAt.Time, FinishedAt: row.FinishedAt.Time,
	}
}

func runFromSessionRow(row dbgen.AIListSessionRunsRow) Run {
	return Run{
		ID: row.ID, AccountID: row.AccountID, DepartmentID: row.DepartmentID, SessionID: row.SessionID,
		Agent: row.Agent, Model: row.Model, InstructionVersion: row.InstructionVersion,
		Question: row.Question, Answer: row.Answer, Status: row.Status, ErrorCode: row.ErrorCode,
		ToolCalls: int(row.ToolCalls), PromptTokens: int(row.PromptTokens), CompletionTokens: int(row.CompletionTokens),
		Duration: time.Duration(row.DurationMs) * time.Millisecond, TraceID: row.TraceID,
		Origin: row.Origin, CorrelationID: row.CorrelationID, CausationID: row.CausationID,
		TriggerDepth: int(row.TriggerDepth), TriggerPolicy: row.TriggerPolicy, EventID: row.EventID,
		Context: json.RawMessage(row.Context), CreatedAt: row.CreatedAt.Time, FinishedAt: row.FinishedAt.Time,
	}
}

func runFromGetRow(row dbgen.AIGetRunRow) Run {
	return Run{
		ID: row.ID, AccountID: row.AccountID, DepartmentID: row.DepartmentID, SessionID: row.SessionID,
		Agent: row.Agent, Model: row.Model, InstructionVersion: row.InstructionVersion,
		Question: row.Question, Answer: row.Answer, Status: row.Status, ErrorCode: row.ErrorCode,
		ToolCalls: int(row.ToolCalls), PromptTokens: int(row.PromptTokens), CompletionTokens: int(row.CompletionTokens),
		Duration: time.Duration(row.DurationMs) * time.Millisecond, TraceID: row.TraceID,
		Origin: row.Origin, CorrelationID: row.CorrelationID, CausationID: row.CausationID,
		TriggerDepth: int(row.TriggerDepth), TriggerPolicy: row.TriggerPolicy, EventID: row.EventID,
		Context: json.RawMessage(row.Context), CreatedAt: row.CreatedAt.Time, FinishedAt: row.FinishedAt.Time,
	}
}
