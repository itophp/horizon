// Package projects implements project collaboration through the capability
// gateway. Projects own members, plans and tasks; membership is the visibility
// boundary and every write goes through one atomic capability.
package projects

import (
	"context"
	"errors"
	"fmt"

	"baseline/services/api/internal/repository/dbgen"
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/gateway"
	"baseline/services/api/kernel/capability/registry"
	"baseline/services/api/system/identity"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Permissions used by the module. The permission decides which action is
// allowed; membership decides which project it may be applied to.
const (
	ReadPermission  = "projects:read"
	WritePermission = "projects:write"
	AdminPermission = "projects:admin"
)

// Success events. One constant per fact keeps the declaration and the executor
// from drifting into different spellings.
const (
	ProjectCreatedEvent  = "project.created"
	ProjectUpdatedEvent  = "project.updated"
	ProjectStatusEvent   = "project.status.changed"
	MemberAddedEvent     = "project.member.added"
	MemberRemovedEvent   = "project.member.removed"
	OwnerTransferred     = "project.owner.transferred"
	PlanCreatedEvent     = "project.plan.created"
	PlanUpdatedEvent     = "project.plan.updated"
	PlanStatusEvent      = "project.plan.status.changed"
	TaskCreatedEvent     = "project.task.created"
	TaskUpdatedEvent     = "project.task.updated"
	TaskStatusEvent      = "project.task.status.changed"
	TaskAssignedEvent    = "project.task.assigned"
	TaskBriefRecordEvent = "project.task.brief.recorded"
)

// Module is the project collaboration module.
type Module struct{ Pool *pgxpool.Pool }

// New builds the module.
func New(pool *pgxpool.Pool) *Module { return &Module{Pool: pool} }

// Entries is the capability catalog contributed by the module.
func (m *Module) Entries() []registry.Entry {
	entries := make([]registry.Entry, 0, len(definitions()))
	for _, definition := range definitions() {
		name := definition.Name
		d := definition
		if d.ReadOnly {
			entries = append(entries, registry.Entry{Definition: d, Handler: contracts.Handler{
				Execute: func(ctx context.Context, in contracts.Input) (contracts.Output, error) {
					return m.read(ctx, dbgen.New(m.Pool), in, name)
				},
			}})
			continue
		}
		entries = append(entries, registry.Entry{Definition: d, Handler: contracts.Handler{
			ExecuteTx: func(ctx context.Context, tx pgx.Tx, in contracts.Input) (contracts.Output, error) {
				return m.write(ctx, dbgen.New(tx), in, name)
			},
		}})
	}
	return entries
}

// read dispatches a read-only capability. The read path never opens a
// transaction, so a long list cannot hold one open.
func (m *Module) read(ctx context.Context, q *dbgen.Queries, in contracts.Input, name string) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	switch name {
	case "project.get":
		return m.getProject(ctx, q, p, in)
	case "project.list":
		return m.listProjects(ctx, q, p, in)
	case "project.member.list":
		return m.listMembers(ctx, q, p, in)
	case "project.member.search":
		return m.searchAccounts(ctx, q, p, in)
	case "project.plan.get":
		return m.getPlan(ctx, q, p, in)
	case "project.plan.list":
		return m.listPlans(ctx, q, p, in)
	case "project.task.get":
		return m.getTask(ctx, q, p, in)
	case "project.task.list":
		return m.listTasks(ctx, q, p, in)
	}
	return contracts.Output{}, contracts.Problem("not_found")
}

// write dispatches a write capability. Everything runs inside the gateway's
// transaction, so the business row and its outbox event commit together.
func (m *Module) write(ctx context.Context, q *dbgen.Queries, in contracts.Input, name string) (contracts.Output, error) {
	p, err := identity.Current(ctx)
	if err != nil {
		return contracts.Output{}, contracts.Problem("unauthenticated")
	}
	switch name {
	case "project.create":
		return m.createProject(ctx, q, p, in)
	case "project.update":
		return m.updateProject(ctx, q, p, in)
	case "project.status.change":
		return m.changeProjectStatus(ctx, q, p, in)
	case "project.member.add":
		return m.addMember(ctx, q, p, in)
	case "project.member.remove":
		return m.removeMember(ctx, q, p, in)
	case "project.owner.transfer":
		return m.transferOwner(ctx, q, p, in)
	case "project.plan.create":
		return m.createPlan(ctx, q, p, in)
	case "project.plan.update":
		return m.updatePlan(ctx, q, p, in)
	case "project.plan.status.change":
		return m.changePlanStatus(ctx, q, p, in)
	case "project.task.create":
		return m.createTask(ctx, q, p, in)
	case "project.task.update":
		return m.updateTask(ctx, q, p, in)
	case "project.task.status.change":
		return m.changeTaskStatus(ctx, q, p, in)
	case "project.task.assign":
		return m.assignTask(ctx, q, p, in)
	case "project.task.brief.record":
		return m.recordBrief(ctx, q, p, in)
	}
	return contracts.Output{}, contracts.Problem("not_found")
}

func definitions() []contracts.Definition {
	return []contracts.Definition{
		createDefinition(),
		getDefinition(),
		listDefinition(),
		updateDefinition(),
		statusDefinition(),
		membersDefinition(),
		memberSearchDefinition(),
		memberAddDefinition(),
		memberRemoveDefinition(),
		ownerTransferDefinition(),
		planCreateDefinition(),
		planGetDefinition(),
		planListDefinition(),
		planUpdateDefinition(),
		planStatusDefinition(),
		taskCreateDefinition(),
		taskGetDefinition(),
		taskListDefinition(),
		taskUpdateDefinition(),
		taskStatusDefinition(),
		taskAssignDefinition(),
		briefRecordDefinition(),
	}
}

func base(name, title, description string, entity string) contracts.Definition {
	return contracts.Definition{
		Layer: contracts.LayerAtomic, Name: name, Version: "1.0.0",
		Title: title, Description: description, Domain: "projects", Entity: entity,
		Risk: contracts.RiskLow, Approval: contracts.ApprovalAuto,
		Executor: "go:projects." + name,
	}
}

func writeBase(name, title, description, entity, permission string, events ...string) contracts.Definition {
	d := base(name, title, description, entity)
	d.Permission = permission
	d.Risk = contracts.RiskMedium
	d.Idempotency = contracts.Idempotency{Required: true, Scope: "tenant+operation+key"}
	d.SuccessEvents = events
	d.Tags = []string{"projects", "write"}
	return d
}

func readBase(name, title, description, entity string) contracts.Definition {
	d := base(name, title, description, entity)
	d.Permission = ReadPermission
	d.ReadOnly = true
	d.TransientResult = true
	d.Tags = []string{"projects", "read"}
	return d
}

// writeProjectID is the shared input of every member and plan capability.
func projectIDSchema() *contracts.Schema { return contracts.Int(1, 1<<53) }

func createDefinition() contracts.Definition {
	d := writeBase("project.create", "Create project",
		"Create a project; the creator becomes its owner and first member. Emits project.created.",
		"project", AdminPermission, ProjectCreatedEvent)
	d.Entities = []string{"project", "project_member"}
	d.Input = contracts.Object(map[string]*contracts.Schema{
		"code":     contracts.Str(1, 32),
		"name":     contracts.Str(1, 120),
		"priority": contracts.Enum(priorities...),
		"dueDate":  dateSchema(),
	}, "code", "name")
	d.Output = contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id")
	d.Loggable = []string{"code", "name", "priority", "dueDate"}
	return d
}

func getDefinition() contracts.Definition {
	d := readBase("project.get", "Inspect project",
		"Read one project with its plan and task counters. A caller without a member row gets not_found.",
		"project")
	d.Input = contracts.Object(map[string]*contracts.Schema{"id": projectIDSchema()}, "id")
	d.Output = contracts.Object(projectProperties(), "id", "code", "name", "status", "priority", "ownerId", "ownerName")
	d.Loggable = []string{"id"}
	return d
}

func listDefinition() contracts.Definition {
	d := readBase("project.list", "List projects",
		"List the projects the caller is a member of, newest first.", "project")
	d.Input = contracts.Object(map[string]*contracts.Schema{
		"status":   contracts.Enum(projectStates...),
		"priority": contracts.Enum(priorities...),
		"keyword":  contracts.Str(0, 120),
		"limit":    contracts.Int(1, 100),
	})
	d.Output = contracts.Object(map[string]*contracts.Schema{
		"items": {Type: "array", Items: contracts.Object(projectProperties(), "id", "code", "name", "status", "priority", "ownerId", "ownerName")},
	}, "items")
	d.Loggable = []string{"status", "priority", "keyword", "limit"}
	return d
}

func updateDefinition() contracts.Definition {
	d := writeBase("project.update", "Update project",
		"Change the name, priority or planned end date. The code is a stable identifier and cannot change.",
		"project", WritePermission, ProjectUpdatedEvent)
	d.Input = contracts.Object(map[string]*contracts.Schema{
		"id":       projectIDSchema(),
		"name":     contracts.Str(1, 120),
		"priority": contracts.Enum(priorities...),
		"dueDate":  dateSchema(),
	}, "id", "name")
	d.Output = contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id")
	d.Loggable = []string{"id", "name", "priority", "dueDate"}
	return d
}

func statusDefinition() contracts.Definition {
	d := writeBase("project.status.change", "Change project status",
		"Move a project between active, paused, completed and archived. Archiving requires the owner and is terminal.",
		"project", WritePermission, ProjectStatusEvent)
	d.Input = contracts.Object(map[string]*contracts.Schema{
		"id":     projectIDSchema(),
		"from":   contracts.Enum(projectStates...),
		"status": contracts.Enum(projectStates...),
	}, "id", "from", "status")
	d.Output = contracts.Object(map[string]*contracts.Schema{
		"id": {Type: "integer"}, "status": {Type: "string"},
	}, "id", "status")
	d.Loggable = []string{"id", "from", "status"}
	return d
}

func membersDefinition() contracts.Definition {
	d := readBase("project.member.list", "List project members",
		"List the members of one project. Only members can read the list.", "project_member")
	d.Input = contracts.Object(map[string]*contracts.Schema{"projectId": projectIDSchema()}, "projectId")
	d.Output = contracts.Object(map[string]*contracts.Schema{
		"items": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
			"accountId": {Type: "integer"}, "name": {Type: "string"}, "email": {Type: "string"},
			"isOwner": {Type: "boolean"},
		}, "accountId", "name", "isOwner")},
	}, "items")
	d.Loggable = []string{"projectId"}
	return d
}

func memberSearchDefinition() contracts.Definition {
	d := readBase("project.member.search", "Search accounts for membership",
		"Search the tenant directory by name or email to pick a member. Returns the minimal projection only.",
		"project_member")
	d.Input = contracts.Object(map[string]*contracts.Schema{
		"projectId": projectIDSchema(),
		"keyword":   contracts.Str(1, 120),
		"limit":     contracts.Int(1, 50),
	}, "projectId", "keyword")
	d.Output = contracts.Object(map[string]*contracts.Schema{
		"items": {Type: "array", Items: contracts.Object(map[string]*contracts.Schema{
			"accountId": {Type: "integer"}, "name": {Type: "string"}, "email": {Type: "string"},
		}, "accountId", "name", "email")},
	}, "items")
	d.Loggable = []string{"projectId", "keyword", "limit"}
	return d
}

func memberAddDefinition() contracts.Definition {
	d := writeBase("project.member.add", "Add project member",
		"Add one account to the project. Only the owner may add members; a repeated add succeeds without a second event.",
		"project_member", WritePermission, MemberAddedEvent)
	d.Input = contracts.Object(map[string]*contracts.Schema{
		"projectId": projectIDSchema(),
		"accountId": projectIDSchema(),
	}, "projectId", "accountId")
	d.Output = contracts.Object(map[string]*contracts.Schema{
		"projectId": {Type: "integer"}, "accountId": {Type: "integer"}, "added": {Type: "boolean"},
	}, "projectId", "accountId", "added")
	d.Loggable = []string{"projectId", "accountId"}
	return d
}

func memberRemoveDefinition() contracts.Definition {
	d := writeBase("project.member.remove", "Remove project member",
		"Remove one member. Only the owner may remove members, and the owner cannot be removed.",
		"project_member", WritePermission, MemberRemovedEvent)
	d.Input = contracts.Object(map[string]*contracts.Schema{
		"projectId": projectIDSchema(),
		"accountId": projectIDSchema(),
	}, "projectId", "accountId")
	d.Output = contracts.Object(map[string]*contracts.Schema{
		"projectId": {Type: "integer"}, "accountId": {Type: "integer"},
	}, "projectId", "accountId")
	d.Loggable = []string{"projectId", "accountId"}
	return d
}

func ownerTransferDefinition() contracts.Definition {
	d := writeBase("project.owner.transfer", "Transfer project ownership",
		"Hand ownership to an existing member. Only the current owner may transfer.",
		"project", WritePermission, OwnerTransferred)
	d.Entities = []string{"project", "project_member"}
	d.Input = contracts.Object(map[string]*contracts.Schema{
		"projectId": projectIDSchema(),
		"accountId": projectIDSchema(),
	}, "projectId", "accountId")
	d.Output = contracts.Object(map[string]*contracts.Schema{
		"projectId": {Type: "integer"}, "ownerId": {Type: "integer"},
	}, "projectId", "ownerId")
	d.Loggable = []string{"projectId", "accountId"}
	return d
}

func planCreateDefinition() contracts.Definition {
	d := writeBase("project.plan.create", "Create project plan",
		"Create a milestone, phase or deliverable with a Markdown body and one responsible member.",
		"project_plan", WritePermission, PlanCreatedEvent)
	d.Input = contracts.Object(map[string]*contracts.Schema{
		"projectId": projectIDSchema(),
		"planType":  contracts.Enum(planTypes...),
		"title":     contracts.Str(1, 200),
		"content":   contracts.Str(0, 20000),
		"priority":  contracts.Enum(priorities...),
		"dueDate":   dateSchema(),
		"ownerId":   projectIDSchema(),
	}, "projectId", "title", "ownerId")
	d.Output = contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id")
	d.Loggable = []string{"projectId", "planType", "title", "priority", "dueDate", "ownerId"}
	return d
}

func planGetDefinition() contracts.Definition {
	d := readBase("project.plan.get", "Inspect project plan",
		"Read one plan including its Markdown body.", "project_plan")
	d.Input = contracts.Object(map[string]*contracts.Schema{"id": projectIDSchema()}, "id")
	d.Output = contracts.Object(planProperties(), "id", "projectId", "planType", "title", "status", "priority", "ownerId", "ownerName")
	d.Loggable = []string{"id"}
	return d
}

func planListDefinition() contracts.Definition {
	d := readBase("project.plan.list", "List project plans",
		"List the plans of one project, newest first.", "project_plan")
	d.Input = contracts.Object(map[string]*contracts.Schema{
		"projectId": projectIDSchema(),
		"limit":     contracts.Int(1, 100),
	}, "projectId")
	d.Output = contracts.Object(map[string]*contracts.Schema{
		"items": {Type: "array", Items: contracts.Object(planProperties(), "id", "projectId", "planType", "title", "status", "priority", "ownerId", "ownerName")},
	}, "items")
	d.Loggable = []string{"projectId", "limit"}
	return d
}

func planUpdateDefinition() contracts.Definition {
	d := writeBase("project.plan.update", "Update project plan",
		"Change the type, title, Markdown body, priority, due date or responsible member of a plan.",
		"project_plan", WritePermission, PlanUpdatedEvent)
	d.Input = contracts.Object(map[string]*contracts.Schema{
		"id":       projectIDSchema(),
		"planType": contracts.Enum(planTypes...),
		"title":    contracts.Str(1, 200),
		"content":  contracts.Str(0, 20000),
		"priority": contracts.Enum(priorities...),
		"dueDate":  dateSchema(),
		"ownerId":  projectIDSchema(),
	}, "id", "title", "ownerId")
	d.Output = contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id")
	d.Loggable = []string{"id", "planType", "title", "priority", "dueDate", "ownerId"}
	return d
}

func planStatusDefinition() contracts.Definition {
	d := writeBase("project.plan.status.change", "Change plan status",
		"Move a plan between pending, in progress, done and cancelled.", "project_plan", WritePermission, PlanStatusEvent)
	d.Input = contracts.Object(map[string]*contracts.Schema{
		"id":     projectIDSchema(),
		"from":   contracts.Enum(planStates...),
		"status": contracts.Enum(planStates...),
	}, "id", "from", "status")
	d.Output = contracts.Object(map[string]*contracts.Schema{
		"id": {Type: "integer"}, "status": {Type: "string"},
	}, "id", "status")
	d.Loggable = []string{"id", "from", "status"}
	return d
}

func taskCreateDefinition() contracts.Definition {
	d := writeBase("project.task.create", "Create project task",
		"Create a task under a project, optionally inside a plan and assigned to a member.",
		"project_task", WritePermission, TaskCreatedEvent)
	d.Input = contracts.Object(map[string]*contracts.Schema{
		"projectId":   projectIDSchema(),
		"planId":      contracts.Int(0, 1<<53),
		"title":       contracts.Str(1, 200),
		"description": contracts.Str(0, 20000),
		"priority":    contracts.Enum(priorities...),
		"assigneeId":  contracts.Int(0, 1<<53),
		"dueDate":     dateSchema(),
	}, "projectId", "title")
	d.Output = contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id")
	d.Loggable = []string{"projectId", "planId", "title", "priority", "assigneeId", "dueDate"}
	return d
}

func taskGetDefinition() contracts.Definition {
	d := readBase("project.task.get", "Inspect project task",
		"Read one task with its Markdown description and the latest AI brief when present.", "project_task")
	d.Input = contracts.Object(map[string]*contracts.Schema{"id": projectIDSchema()}, "id")
	properties := taskProperties()
	properties["brief"] = &contracts.Schema{Type: "string"}
	properties["briefModel"] = &contracts.Schema{Type: "string"}
	d.Output = contracts.Object(properties, "id", "projectId", "title", "status", "priority")
	d.Loggable = []string{"id"}
	return d
}

func taskListDefinition() contracts.Definition {
	d := readBase("project.task.list", "List project tasks",
		"List the tasks of one project, filtered by plan, assignee or status.", "project_task")
	d.Input = contracts.Object(map[string]*contracts.Schema{
		"projectId":  projectIDSchema(),
		"planId":     contracts.Int(0, 1<<53),
		"assigneeId": contracts.Int(0, 1<<53),
		"status":     contracts.Enum(taskStates...),
		"limit":      contracts.Int(1, 100),
	}, "projectId")
	d.Output = contracts.Object(map[string]*contracts.Schema{
		"items": {Type: "array", Items: contracts.Object(taskProperties(), "id", "projectId", "title", "status", "priority")},
	}, "items")
	d.Loggable = []string{"projectId", "planId", "assigneeId", "status", "limit"}
	return d
}

func taskUpdateDefinition() contracts.Definition {
	d := writeBase("project.task.update", "Update project task",
		"Change the plan, title, Markdown description, priority, assignee or due date of a task.",
		"project_task", WritePermission, TaskUpdatedEvent)
	d.Input = contracts.Object(map[string]*contracts.Schema{
		"id":          projectIDSchema(),
		"planId":      contracts.Int(0, 1<<53),
		"title":       contracts.Str(1, 200),
		"description": contracts.Str(0, 20000),
		"priority":    contracts.Enum(priorities...),
		"assigneeId":  contracts.Int(0, 1<<53),
		"dueDate":     dateSchema(),
	}, "id", "title")
	d.Output = contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id")
	d.Loggable = []string{"id", "planId", "title", "priority", "assigneeId", "dueDate"}
	return d
}

func taskStatusDefinition() contracts.Definition {
	d := writeBase("project.task.status.change", "Change task status",
		"Move a task between todo, in progress, done and cancelled. Done sets completedAt, leaving done clears it.",
		"project_task", WritePermission, TaskStatusEvent)
	d.Input = contracts.Object(map[string]*contracts.Schema{
		"id":     projectIDSchema(),
		"from":   contracts.Enum(taskStates...),
		"status": contracts.Enum(taskStates...),
	}, "id", "from", "status")
	d.Output = contracts.Object(map[string]*contracts.Schema{
		"id": {Type: "integer"}, "status": {Type: "string"}, "completedAt": {Type: "string"},
	}, "id", "status")
	d.Loggable = []string{"id", "from", "status"}
	return d
}

func taskAssignDefinition() contracts.Definition {
	d := writeBase("project.task.assign", "Assign project task",
		"Assign a task to a project member. Emits project.task.assigned, which drives the AI brief.",
		"project_task", WritePermission, TaskAssignedEvent)
	d.Input = contracts.Object(map[string]*contracts.Schema{
		"id":         projectIDSchema(),
		"assigneeId": projectIDSchema(),
	}, "id", "assigneeId")
	d.Output = contracts.Object(map[string]*contracts.Schema{
		"id": {Type: "integer"}, "assigneeId": {Type: "integer"},
	}, "id", "assigneeId")
	d.Loggable = []string{"id", "assigneeId"}
	return d
}

func briefRecordDefinition() contracts.Definition {
	d := writeBase("project.task.brief.record", "Record task brief",
		"Store the AI handover brief for a task. Only an agent or workflow may call this: a person cannot hand-write a model conclusion.",
		"project_task_brief", WritePermission, TaskBriefRecordEvent)
	d.Entities = []string{"project_task_brief", "project_task"}
	d.Risk = contracts.RiskLow
	d.Input = contracts.Object(map[string]*contracts.Schema{
		"taskId":  projectIDSchema(),
		"summary": contracts.Str(1, 4000),
		"model":   contracts.Str(0, 80),
		"runId":   contracts.Int(0, 1<<53),
	}, "taskId", "summary")
	d.Output = contracts.Object(map[string]*contracts.Schema{"id": {Type: "integer"}}, "id")
	// The summary is model output derived from a sensitive body; keep it out of
	// the audit projection.
	d.Loggable = []string{"taskId", "runId"}
	return d
}

func dateSchema() *contracts.Schema { return contracts.Str(0, 10) }

func problem(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505", "23503":
			return contracts.Problem("conflict")
		case "23514":
			return contracts.Problem("invalid")
		}
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return contracts.Problem("not_found")
	}
	return err
}

func event(name, aggregate string, id int64, payload map[string]any) []contracts.Event {
	return []contracts.Event{{
		Type: name, AggregateType: aggregate, AggregateID: fmt.Sprint(id), Payload: payload,
	}}
}

// invocationOrigin reports where the current call came from. Executors use it to
// keep a fact (a recorded AI verdict) from being produced by a page.
func invocationOrigin(ctx context.Context) contracts.Origin {
	meta, ok := gateway.CurrentInvocation(ctx)
	if !ok {
		return ""
	}
	return meta.Origin
}
