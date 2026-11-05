package projects

import (
	"context"
	"errors"
	"fmt"

	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/system/identity"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// taskTransitions maps a target state to the states it may be entered from.
var taskTransitions = map[string][]string{
	"in_progress": {"todo"},
	"done":        {"in_progress"},
	"todo":        {"done"},
	"cancelled":   {"todo", "in_progress"},
}

func (m *Module) createTask(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	projectID := integer(in, "projectId")
	title := text(in, "title")
	if projectID < 1 || title == "" {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	priority := text(in, "priority")
	if priority == "" {
		priority = "normal"
	}
	if !allowed(priority, priorities...) {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	due, err := dateValue(in, "dueDate")
	if err != nil {
		return contracts.Output{}, err
	}
	if _, err = q.ProjectGet(ctx, dbgen.ProjectGetParams{TenantID: p.TenantID, ID: projectID, AccountID: p.Account.ID}); err != nil {
		return contracts.Output{}, problem(err)
	}
	planID, assigneeID := nullableID(in, "planId"), nullableID(in, "assigneeId")
	if err = m.checkPlan(ctx, q, p, projectID, planID); err != nil {
		return contracts.Output{}, err
	}
	if err = m.checkAssignee(ctx, q, p, projectID, assigneeID); err != nil {
		return contracts.Output{}, err
	}
	id, err := q.ProjectTaskCreate(ctx, dbgen.ProjectTaskCreateParams{
		TenantID: p.TenantID, ProjectID: projectID, PlanID: planID, Title: title,
		Description: text(in, "description"), Priority: priority, AssigneeID: assigneeID,
		DueDate: due, CreatedBy: p.Account.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Everything else was checked above; an archived project is the reason.
		return contracts.Output{}, contracts.Problem("conflict")
	}
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	return contracts.Output{
		Data:    map[string]any{"id": id},
		Summary: fmt.Sprintf("task=%d project=%d", id, projectID),
		Events: event(TaskCreatedEvent, "project_task", id, map[string]any{
			"taskId": id, "projectId": projectID, "title": title,
		}),
	}, nil
}

func (m *Module) getTask(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	row, err := q.ProjectTaskGet(ctx, dbgen.ProjectTaskGetParams{
		TenantID: p.TenantID, ID: integer(in, "id"), AccountID: p.Account.ID,
	})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	data := taskData(row)
	brief, model := "", ""
	record, err := q.ProjectTaskBriefGet(ctx, dbgen.ProjectTaskBriefGetParams{TenantID: p.TenantID, TaskID: row.ID})
	switch {
	case err == nil:
		brief, model = record.Summary, record.Model
	case !errors.Is(err, pgx.ErrNoRows):
		return contracts.Output{}, err
	}
	data["brief"], data["briefModel"] = brief, model
	return contracts.Output{Data: data, Summary: fmt.Sprintf("task=%d", row.ID)}, nil
}

func (m *Module) listTasks(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	projectID := integer(in, "projectId")
	status := text(in, "status")
	if status != "" && !allowed(status, taskStates...) {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	if _, err := q.ProjectGet(ctx, dbgen.ProjectGetParams{TenantID: p.TenantID, ID: projectID, AccountID: p.Account.ID}); err != nil {
		return contracts.Output{}, problem(err)
	}
	page, size := paging(in, 50)
	rows, err := q.ProjectTaskList(ctx, dbgen.ProjectTaskListParams{
		TenantID: p.TenantID, ProjectID: projectID, AccountID: p.Account.ID,
		PlanFilter: integer(in, "planId"), AssigneeFilter: integer(in, "assigneeId"),
		StatusFilter: status, MaxRows: size, PageOffset: (page - 1) * size, Keyword: text(in, "keyword"),
	})
	if err != nil {
		return contracts.Output{}, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"id": row.ID, "projectId": row.ProjectID,
			"planId": idText(row.PlanID), "planTitle": optText(row.PlanTitle),
			"title": row.Title, "status": row.Status, "priority": row.Priority,
			"assigneeId": idText(row.AssigneeID), "assigneeName": optText(row.AssigneeName),
			"dueDate": dateText(row.DueDate), "completedAt": timeText(row.CompletedAt),
		})
	}
	total, err := q.ProjectTaskCount(ctx, dbgen.ProjectTaskCountParams{TenantID: p.TenantID, ProjectID: projectID, AccountID: p.Account.ID, PlanFilter: integer(in, "planId"), AssigneeFilter: integer(in, "assigneeId"), StatusFilter: status, Keyword: text(in, "keyword")})
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"items": items, "total": total, "page": page, "pageSize": size}, Summary: fmt.Sprintf("tasks=%d", len(items))}, nil
}

func (m *Module) updateTask(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	id, title := integer(in, "id"), text(in, "title")
	if id < 1 || title == "" {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	priority := text(in, "priority")
	if priority == "" {
		priority = "normal"
	}
	if !allowed(priority, priorities...) {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	due, err := dateValue(in, "dueDate")
	if err != nil {
		return contracts.Output{}, err
	}
	current, err := q.ProjectTaskGet(ctx, dbgen.ProjectTaskGetParams{TenantID: p.TenantID, ID: id, AccountID: p.Account.ID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	planID, assigneeID := nullableID(in, "planId"), nullableID(in, "assigneeId")
	if err = m.checkPlan(ctx, q, p, current.ProjectID, planID); err != nil {
		return contracts.Output{}, err
	}
	if err = m.checkAssignee(ctx, q, p, current.ProjectID, assigneeID); err != nil {
		return contracts.Output{}, err
	}
	rows, err := q.ProjectTaskUpdate(ctx, dbgen.ProjectTaskUpdateParams{
		PlanID: planID, Title: title, Description: text(in, "description"), Priority: priority,
		AssigneeID: assigneeID, DueDate: due,
		TenantID: p.TenantID, ID: id, ActorID: p.Account.ID,
	})
	if err != nil {
		return contracts.Output{}, err
	}
	if rows != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return contracts.Output{
		Data:    map[string]any{"id": id},
		Summary: fmt.Sprintf("task=%d", id),
		Events: event(TaskUpdatedEvent, "project_task", id, map[string]any{
			"taskId": id, "projectId": current.ProjectID, "title": title,
		}),
	}, nil
}

func (m *Module) changeTaskStatus(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	id, from, to := integer(in, "id"), text(in, "from"), text(in, "status")
	if id < 1 || !allowed(from, taskStates...) || !allowed(to, taskStates...) {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	if !reachable(taskTransitions, from, to) {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	rows, err := q.ProjectTaskStatusChange(ctx, dbgen.ProjectTaskStatusChangeParams{
		ToStatus: to, TenantID: p.TenantID, ID: id, FromStatus: from, ActorID: p.Account.ID,
	})
	if err != nil {
		return contracts.Output{}, err
	}
	if rows != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	completed := ""
	if to == "done" {
		// The exact timestamp lives in the row; the caller only needs to know it
		// was set, so the current value is read back.
		if row, err := q.ProjectTaskGet(ctx, dbgen.ProjectTaskGetParams{TenantID: p.TenantID, ID: id, AccountID: p.Account.ID}); err == nil {
			completed = timeText(row.CompletedAt)
		}
	}
	return contracts.Output{
		Data:    map[string]any{"id": id, "status": to, "completedAt": completed},
		Summary: fmt.Sprintf("task=%d status=%s", id, to),
		Events:  event(TaskStatusEvent, "project_task", id, map[string]any{"taskId": id, "from": from, "status": to}),
	}, nil
}

func (m *Module) assignTask(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	id, assigneeID := integer(in, "id"), integer(in, "assigneeId")
	if id < 1 || assigneeID < 1 {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	current, err := q.ProjectTaskGet(ctx, dbgen.ProjectTaskGetParams{TenantID: p.TenantID, ID: id, AccountID: p.Account.ID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	member, err := q.ProjectMemberExists(ctx, dbgen.ProjectMemberExistsParams{
		TenantID: p.TenantID, ProjectID: current.ProjectID, AccountID: assigneeID,
	})
	if err != nil {
		return contracts.Output{}, err
	}
	if !member {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	if idText(current.AssigneeID) == assigneeID {
		// Assigning the same person changes nothing, so it cannot report
		// success: that would demand a second assignment event and therefore a
		// second AI brief.
		return contracts.Output{}, contracts.Problem("conflict")
	}
	rows, err := q.ProjectTaskAssign(ctx, dbgen.ProjectTaskAssignParams{
		AssigneeID: pgtype.Int8{Int64: assigneeID, Valid: true}, TenantID: p.TenantID, ID: id, ActorID: p.Account.ID,
	})
	if err != nil {
		return contracts.Output{}, err
	}
	if rows != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return contracts.Output{
		Data:    map[string]any{"id": id, "assigneeId": assigneeID},
		Summary: fmt.Sprintf("task=%d assignee=%d", id, assigneeID),
		Events: event(TaskAssignedEvent, "project_task", id, map[string]any{
			"taskId": id, "projectId": current.ProjectID, "assigneeId": assigneeID, "title": current.Title,
		}),
	}, nil
}

// recordBrief is the write-back channel for AI output. Human and plain API callers
// cannot claim that a model produced a conclusion.
func (m *Module) recordBrief(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	switch invocationOrigin(ctx) {
	case contracts.OriginAgent, contracts.OriginWorkflow:
	default:
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	taskID, summary := integer(in, "taskId"), text(in, "summary")
	if taskID < 1 || summary == "" {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	if _, err := q.ProjectTaskGet(ctx, dbgen.ProjectTaskGetParams{TenantID: p.TenantID, ID: taskID, AccountID: p.Account.ID}); err != nil {
		return contracts.Output{}, problem(err)
	}
	id, err := q.ProjectTaskBriefUpsert(ctx, dbgen.ProjectTaskBriefUpsertParams{
		TenantID: p.TenantID, TaskID: taskID, RunID: integer(in, "runId"),
		Summary: summary, Model: text(in, "model"),
	})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	return contracts.Output{
		Data:    map[string]any{"id": id},
		Summary: fmt.Sprintf("brief=%d task=%d", id, taskID),
		Events: event(TaskBriefRecordEvent, "project_task_brief", id, map[string]any{
			"briefId": id, "taskId": taskID,
		}),
	}, nil
}

// checkPlan rejects a plan reference that belongs to another project.
func (m *Module) checkPlan(ctx context.Context, q *dbgen.Queries, p *identity.Principal, projectID int64, planID pgtype.Int8) error {
	if !planID.Valid {
		return nil
	}
	plan, err := q.ProjectPlanGet(ctx, dbgen.ProjectPlanGetParams{
		TenantID: p.TenantID, ID: planID.Int64, AccountID: p.Account.ID,
	})
	if err != nil {
		return problem(err)
	}
	if plan.ProjectID != projectID {
		return contracts.Problem("invalid")
	}
	return nil
}

// checkAssignee rejects an assignee that is not a member of the project.
func (m *Module) checkAssignee(ctx context.Context, q *dbgen.Queries, p *identity.Principal, projectID int64, assigneeID pgtype.Int8) error {
	if !assigneeID.Valid {
		return nil
	}
	member, err := q.ProjectMemberExists(ctx, dbgen.ProjectMemberExistsParams{
		TenantID: p.TenantID, ProjectID: projectID, AccountID: assigneeID.Int64,
	})
	if err != nil {
		return err
	}
	if !member {
		return contracts.Problem("invalid")
	}
	return nil
}

func taskData(row dbgen.ProjectTaskGetRow) map[string]any {
	return map[string]any{
		"id": row.ID, "projectId": row.ProjectID,
		"planId": idText(row.PlanID), "planTitle": optText(row.PlanTitle),
		"title": row.Title, "description": row.Description, "status": row.Status,
		"priority": row.Priority, "assigneeId": idText(row.AssigneeID),
		"assigneeName": optText(row.AssigneeName), "dueDate": dateText(row.DueDate),
		"completedAt": timeText(row.CompletedAt),
		"createdAt":   timeText(row.CreatedAt), "updatedAt": timeText(row.UpdatedAt),
	}
}
