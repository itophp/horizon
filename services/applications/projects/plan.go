package projects

import (
	"context"
	"errors"
	"fmt"

	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/system/identity"

	"github.com/jackc/pgx/v5"
)

// planTransitions maps a target state to the states it may be entered from.
var planTransitions = map[string][]string{
	"in_progress": {"pending", "done"},
	"done":        {"in_progress"},
	"cancelled":   {"pending", "in_progress"},
}

func (m *Module) createPlan(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	projectID, ownerID := integer(in, "projectId"), integer(in, "ownerId")
	title := text(in, "title")
	planType := text(in, "planType")
	if planType == "" {
		planType = "phase"
	}
	if projectID < 1 || ownerID < 1 || title == "" || !allowed(planType, planTypes...) {
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
	// Membership of the caller and of the responsible member are both required;
	// the first is answered as not_found so a project id cannot be probed.
	if _, err = q.ProjectGet(ctx, dbgen.ProjectGetParams{TenantID: p.TenantID, ID: projectID, AccountID: p.Account.ID}); err != nil {
		return contracts.Output{}, problem(err)
	}
	member, err := q.ProjectMemberExists(ctx, dbgen.ProjectMemberExistsParams{
		TenantID: p.TenantID, ProjectID: projectID, AccountID: ownerID,
	})
	if err != nil {
		return contracts.Output{}, err
	}
	if !member {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	id, err := q.ProjectPlanCreate(ctx, dbgen.ProjectPlanCreateParams{
		TenantID: p.TenantID, ProjectID: projectID, PlanType: planType, Title: title,
		Content: text(in, "content"), Priority: priority, DueDate: due,
		OwnerID: ownerID, CreatedBy: p.Account.ID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		// Membership and the responsible member were checked above, so the only
		// remaining reason for no row is an archived project.
		return contracts.Output{}, contracts.Problem("conflict")
	}
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	return contracts.Output{
		Data:    map[string]any{"id": id},
		Summary: fmt.Sprintf("plan=%d project=%d", id, projectID),
		Events: event(PlanCreatedEvent, "project_plan", id, map[string]any{
			"planId": id, "projectId": projectID, "title": title, "ownerId": ownerID,
		}),
	}, nil
}

func (m *Module) getPlan(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	row, err := q.ProjectPlanGet(ctx, dbgen.ProjectPlanGetParams{
		TenantID: p.TenantID, ID: integer(in, "id"), AccountID: p.Account.ID,
	})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	return contracts.Output{
		Data: map[string]any{
			"id": row.ID, "projectId": row.ProjectID, "planType": row.PlanType,
			"title": row.Title, "content": row.Content, "status": row.Status,
			"priority": row.Priority, "dueDate": dateText(row.DueDate),
			"ownerId": row.OwnerID, "ownerName": optText(row.OwnerName),
			"createdAt": timeText(row.CreatedAt), "updatedAt": timeText(row.UpdatedAt),
		},
		Summary: fmt.Sprintf("plan=%d", row.ID),
	}, nil
}

func (m *Module) listPlans(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	projectID := integer(in, "projectId")
	if _, err := q.ProjectGet(ctx, dbgen.ProjectGetParams{TenantID: p.TenantID, ID: projectID, AccountID: p.Account.ID}); err != nil {
		return contracts.Output{}, problem(err)
	}
	rows, err := q.ProjectPlanList(ctx, dbgen.ProjectPlanListParams{
		TenantID: p.TenantID, ProjectID: projectID, AccountID: p.Account.ID,
		MaxRows: boundedLimit(in, 50, 100),
	})
	if err != nil {
		return contracts.Output{}, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"id": row.ID, "projectId": row.ProjectID, "planType": row.PlanType,
			"title": row.Title, "status": row.Status, "priority": row.Priority,
			"dueDate": dateText(row.DueDate), "ownerId": row.OwnerID, "ownerName": optText(row.OwnerName),
			"createdAt": timeText(row.CreatedAt), "updatedAt": timeText(row.UpdatedAt),
		})
	}
	return contracts.Output{Data: map[string]any{"items": items}, Summary: fmt.Sprintf("plans=%d", len(items))}, nil
}

func (m *Module) updatePlan(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	id, ownerID := integer(in, "id"), integer(in, "ownerId")
	title := text(in, "title")
	if id < 1 || ownerID < 1 || title == "" {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	planType := text(in, "planType")
	if planType == "" {
		planType = "phase"
	}
	priority := text(in, "priority")
	if priority == "" {
		priority = "normal"
	}
	if !allowed(planType, planTypes...) || !allowed(priority, priorities...) {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	due, err := dateValue(in, "dueDate")
	if err != nil {
		return contracts.Output{}, err
	}
	current, err := q.ProjectPlanGet(ctx, dbgen.ProjectPlanGetParams{TenantID: p.TenantID, ID: id, AccountID: p.Account.ID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	member, err := q.ProjectMemberExists(ctx, dbgen.ProjectMemberExistsParams{
		TenantID: p.TenantID, ProjectID: current.ProjectID, AccountID: ownerID,
	})
	if err != nil {
		return contracts.Output{}, err
	}
	if !member {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	rows, err := q.ProjectPlanUpdate(ctx, dbgen.ProjectPlanUpdateParams{
		PlanType: planType, Title: title, Content: text(in, "content"), Priority: priority,
		DueDate: due, OwnerID: ownerID, TenantID: p.TenantID, ID: id, ActorID: p.Account.ID,
	})
	if err != nil {
		return contracts.Output{}, err
	}
	if rows != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return contracts.Output{
		Data:    map[string]any{"id": id},
		Summary: fmt.Sprintf("plan=%d", id),
		Events: event(PlanUpdatedEvent, "project_plan", id, map[string]any{
			"planId": id, "projectId": current.ProjectID, "title": title, "ownerId": ownerID,
		}),
	}, nil
}

func (m *Module) changePlanStatus(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	id, from, to := integer(in, "id"), text(in, "from"), text(in, "status")
	if id < 1 || !allowed(from, planStates...) || !allowed(to, planStates...) {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	if !reachable(planTransitions, from, to) {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	rows, err := q.ProjectPlanStatusChange(ctx, dbgen.ProjectPlanStatusChangeParams{
		ToStatus: to, TenantID: p.TenantID, ID: id, FromStatus: from, ActorID: p.Account.ID,
	})
	if err != nil {
		return contracts.Output{}, err
	}
	if rows != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return contracts.Output{
		Data:    map[string]any{"id": id, "status": to},
		Summary: fmt.Sprintf("plan=%d status=%s", id, to),
		Events:  event(PlanStatusEvent, "project_plan", id, map[string]any{"planId": id, "from": from, "status": to}),
	}, nil
}
