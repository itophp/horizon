package projects

import (
	"context"
	"fmt"

	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/system/identity"

	"github.com/jackc/pgx/v5/pgtype"
)

// projectTransitions maps a target state to the states it may be entered from.
// The same table is declared in entity_project.go; keeping it here lets the
// executor reject an illegal edge before it reaches SQL.
var projectTransitions = map[string][]string{
	"active":    {"paused"},
	"paused":    {"active"},
	"completed": {"active", "paused"},
	"archived":  {"active", "paused", "completed"},
}

func (m *Module) createProject(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	code, name := text(in, "code"), text(in, "name")
	if code == "" || name == "" {
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
	id, err := q.ProjectCreate(ctx, dbgen.ProjectCreateParams{
		TenantID: p.TenantID, Code: code, Name: name, Priority: priority,
		DueDate: due, OwnerID: p.Account.ID, CreatedBy: p.Account.ID,
	})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	// The creator is the owner, so the membership insert satisfies its own
	// owner check. If it ever fails the transaction rolls the project back.
	if _, err = q.ProjectMemberAdd(ctx, dbgen.ProjectMemberAddParams{
		TenantID: p.TenantID, ProjectID: id, AccountID: p.Account.ID, ActorID: p.Account.ID,
	}); err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{
		Data:    map[string]any{"id": id},
		Summary: fmt.Sprintf("project=%d", id),
		Events: event(ProjectCreatedEvent, "project", id, map[string]any{
			"projectId": id, "code": code, "name": name, "ownerId": p.Account.ID,
		}),
	}, nil
}

func (m *Module) getProject(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	row, err := q.ProjectGet(ctx, dbgen.ProjectGetParams{
		TenantID: p.TenantID, ID: integer(in, "id"), AccountID: p.Account.ID,
	})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	return contracts.Output{Data: projectData(row), Summary: fmt.Sprintf("project=%d", row.ID)}, nil
}

func (m *Module) listProjects(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	status, priority := text(in, "status"), text(in, "priority")
	if status != "" && !allowed(status, projectStates...) {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	if priority != "" && !allowed(priority, priorities...) {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	page, size := paging(in, 30)
	sortKey := text(in, "sort")
	if sortKey == "" {
		sortKey = "id:desc"
	}
	rows, err := q.ProjectList(ctx, dbgen.ProjectListParams{
		TenantID: p.TenantID, AccountID: p.Account.ID,
		StatusFilter: status, PriorityFilter: priority, Keyword: text(in, "keyword"),
		MaxRows: size, PageOffset: (page - 1) * size, SortKey: sortKey,
	})
	if err != nil {
		return contracts.Output{}, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, projectListData(row))
	}
	total, err := q.ProjectCount(ctx, dbgen.ProjectCountParams{TenantID: p.TenantID, AccountID: p.Account.ID, StatusFilter: status, PriorityFilter: priority, Keyword: text(in, "keyword")})
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: map[string]any{"items": items, "total": total, "page": page, "pageSize": size}, Summary: fmt.Sprintf("projects=%d", len(items))}, nil
}

func (m *Module) updateProject(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	id, name := integer(in, "id"), text(in, "name")
	if id < 1 || name == "" {
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
	rows, err := q.ProjectUpdate(ctx, dbgen.ProjectUpdateParams{
		Name: name, Priority: priority, DueDate: due,
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
		Summary: fmt.Sprintf("project=%d", id),
		Events:  event(ProjectUpdatedEvent, "project", id, map[string]any{"projectId": id, "name": name}),
	}, nil
}

func (m *Module) changeProjectStatus(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	id, from, to := integer(in, "id"), text(in, "from"), text(in, "status")
	if id < 1 || !allowed(from, projectStates...) || !allowed(to, projectStates...) {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	if !reachable(projectTransitions, from, to) {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	// Archiving is the owner's decision; pausing and completing are not.
	requireOwner := to == "archived"
	rows, err := q.ProjectStatusChange(ctx, dbgen.ProjectStatusChangeParams{
		ToStatus: to, TenantID: p.TenantID, ID: id, FromStatus: from,
		ActorID: p.Account.ID, RequireOwner: requireOwner,
	})
	if err != nil {
		return contracts.Output{}, err
	}
	if rows != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return contracts.Output{
		Data:    map[string]any{"id": id, "status": to},
		Summary: fmt.Sprintf("project=%d status=%s", id, to),
		Events:  event(ProjectStatusEvent, "project", id, map[string]any{"projectId": id, "from": from, "status": to}),
	}, nil
}

func (m *Module) listMembers(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	projectID := integer(in, "projectId")
	if _, err := q.ProjectGet(ctx, dbgen.ProjectGetParams{TenantID: p.TenantID, ID: projectID, AccountID: p.Account.ID}); err != nil {
		return contracts.Output{}, problem(err)
	}
	rows, err := q.ProjectMemberList(ctx, dbgen.ProjectMemberListParams{
		TenantID: p.TenantID, ProjectID: projectID, AccountID: p.Account.ID, MaxRows: 100, Keyword: text(in, "keyword"),
	})
	if err != nil {
		return contracts.Output{}, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{
			"accountId": row.AccountID, "name": row.Name, "email": row.Email, "isOwner": row.IsOwner,
		})
	}
	return contracts.Output{Data: map[string]any{"items": items}, Summary: fmt.Sprintf("members=%d", len(items))}, nil
}

// searchAccounts is the only way a non-admin finds people to add: it returns the
// minimal projection and requires the caller to be a member of the project.
func (m *Module) searchAccounts(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	projectID := integer(in, "projectId")
	keyword := text(in, "keyword")
	if keyword == "" {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	if _, err := q.ProjectGet(ctx, dbgen.ProjectGetParams{TenantID: p.TenantID, ID: projectID, AccountID: p.Account.ID}); err != nil {
		return contracts.Output{}, problem(err)
	}
	rows, err := q.AccountSearch(ctx, dbgen.AccountSearchParams{
		TenantID: p.TenantID, Keyword: pgtype.Text{String: keyword, Valid: true}, MaxRows: boundedLimit(in, 20, 50),
	})
	if err != nil {
		return contracts.Output{}, err
	}
	items := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]any{"accountId": row.ID, "name": row.Name, "email": row.Email})
	}
	return contracts.Output{Data: map[string]any{"items": items}, Summary: fmt.Sprintf("accounts=%d", len(items))}, nil
}

func (m *Module) addMember(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	projectID, accountID := integer(in, "projectId"), integer(in, "accountId")
	if projectID < 1 || accountID < 1 {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	if _, err := q.ProjectGet(ctx, dbgen.ProjectGetParams{TenantID: p.TenantID, ID: projectID, AccountID: p.Account.ID}); err != nil {
		return contracts.Output{}, problem(err)
	}
	owner, err := q.ProjectOwnerGet(ctx, dbgen.ProjectOwnerGetParams{TenantID: p.TenantID, ID: projectID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	if owner.Status == "archived" {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	if owner.OwnerID != p.Account.ID {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	exists, err := q.ProjectMemberExists(ctx, dbgen.ProjectMemberExistsParams{
		TenantID: p.TenantID, ProjectID: projectID, AccountID: accountID,
	})
	if err != nil {
		return contracts.Output{}, err
	}
	if exists {
		// A write that changes nothing cannot report success: the platform
		// requires every successful write to produce a fact, and a second
		// "member added" event would be a duplicate. Retrying the same request
		// is handled by the idempotency key, not here.
		return contracts.Output{}, contracts.Problem("conflict")
	}
	rows, err := q.ProjectMemberAdd(ctx, dbgen.ProjectMemberAddParams{
		TenantID: p.TenantID, ProjectID: projectID, AccountID: accountID, ActorID: p.Account.ID,
	})
	if err != nil {
		return contracts.Output{}, err
	}
	if rows != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return contracts.Output{
		Data:    map[string]any{"projectId": projectID, "accountId": accountID, "added": true},
		Summary: fmt.Sprintf("project=%d member=%d", projectID, accountID),
		Events:  event(MemberAddedEvent, "project_member", projectID, map[string]any{"projectId": projectID, "accountId": accountID}),
	}, nil
}

func (m *Module) removeMember(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	projectID, accountID := integer(in, "projectId"), integer(in, "accountId")
	if projectID < 1 || accountID < 1 {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	if _, err := q.ProjectGet(ctx, dbgen.ProjectGetParams{TenantID: p.TenantID, ID: projectID, AccountID: p.Account.ID}); err != nil {
		return contracts.Output{}, problem(err)
	}
	owner, err := q.ProjectOwnerGet(ctx, dbgen.ProjectOwnerGetParams{TenantID: p.TenantID, ID: projectID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	if owner.Status == "archived" {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	if owner.OwnerID != p.Account.ID {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	if owner.OwnerID == accountID {
		// The owner must hand over before leaving; otherwise the project would
		// have no owner and no member able to fix it.
		return contracts.Output{}, contracts.Problem("conflict")
	}
	rows, err := q.ProjectMemberRemove(ctx, dbgen.ProjectMemberRemoveParams{
		TenantID: p.TenantID, ProjectID: projectID, AccountID: accountID, ActorID: p.Account.ID,
	})
	if err != nil {
		return contracts.Output{}, err
	}
	if rows != 1 {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return contracts.Output{
		Data:    map[string]any{"projectId": projectID, "accountId": accountID},
		Summary: fmt.Sprintf("project=%d removed=%d", projectID, accountID),
		Events:  event(MemberRemovedEvent, "project_member", projectID, map[string]any{"projectId": projectID, "accountId": accountID}),
	}, nil
}

func (m *Module) transferOwner(ctx context.Context, q *dbgen.Queries, p *identity.Principal, in contracts.Input) (contracts.Output, error) {
	projectID, accountID := integer(in, "projectId"), integer(in, "accountId")
	if projectID < 1 || accountID < 1 {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	if _, err := q.ProjectGet(ctx, dbgen.ProjectGetParams{TenantID: p.TenantID, ID: projectID, AccountID: p.Account.ID}); err != nil {
		return contracts.Output{}, problem(err)
	}
	owner, err := q.ProjectOwnerGet(ctx, dbgen.ProjectOwnerGetParams{TenantID: p.TenantID, ID: projectID})
	if err != nil {
		return contracts.Output{}, problem(err)
	}
	if owner.Status == "archived" {
		return contracts.Output{}, contracts.Problem("conflict")
	}
	if owner.OwnerID != p.Account.ID {
		return contracts.Output{}, contracts.Problem("forbidden")
	}
	if owner.OwnerID == accountID {
		return contracts.Output{}, contracts.Problem("invalid")
	}
	rows, err := q.ProjectOwnerSet(ctx, dbgen.ProjectOwnerSetParams{
		NewOwnerID: accountID, TenantID: p.TenantID, ID: projectID, ActorID: p.Account.ID,
	})
	if err != nil {
		return contracts.Output{}, err
	}
	if rows != 1 {
		// The new owner is not a member yet.
		return contracts.Output{}, contracts.Problem("conflict")
	}
	return contracts.Output{
		Data:    map[string]any{"projectId": projectID, "ownerId": accountID},
		Summary: fmt.Sprintf("project=%d owner=%d", projectID, accountID),
		Events: event(OwnerTransferred, "project", projectID, map[string]any{
			"projectId": projectID, "ownerId": accountID, "previousOwnerId": owner.OwnerID,
		}),
	}, nil
}

// reachable reports whether one declared transition allows from -> to.
func reachable(transitions map[string][]string, from, to string) bool {
	for _, candidate := range transitions[to] {
		if candidate == from {
			return true
		}
	}
	return false
}

func projectData(row dbgen.ProjectGetRow) map[string]any {
	return map[string]any{
		"id": row.ID, "code": row.Code, "name": row.Name, "status": row.Status,
		"priority": row.Priority, "dueDate": dateText(row.DueDate),
		"ownerId": row.OwnerID, "ownerName": optText(row.OwnerName),
		"planCount": row.PlanCount, "taskCount": row.TaskCount,
		"taskDoneCount": row.TaskDoneCount, "taskOverdueCount": row.TaskOverdueCount,
		"createdAt": timeText(row.CreatedAt), "updatedAt": timeText(row.UpdatedAt),
	}
}

func projectListData(row dbgen.ProjectListRow) map[string]any {
	return map[string]any{
		"id": row.ID, "code": row.Code, "name": row.Name, "status": row.Status,
		"priority": row.Priority, "dueDate": dateText(row.DueDate),
		"ownerId": row.OwnerID, "ownerName": optText(row.OwnerName),
		"planCount": row.PlanCount, "taskCount": row.TaskCount,
		"taskDoneCount": row.TaskDoneCount, "taskOverdueCount": row.TaskOverdueCount,
		"createdAt": timeText(row.CreatedAt), "updatedAt": timeText(row.UpdatedAt),
	}
}
