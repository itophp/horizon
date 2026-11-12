//go:build integration

package tests

import (
	"fmt"
	"horizon/foundation/kernel/capability/contracts"
	"testing"
)

func TestRuntimeV2BatchAssignmentAndPagination(t *testing.T) {
	h := newProjectHarness(t)
	owner := h.account(t, "runtime-owner", "projects:read", "projects:write", "projects:admin")
	outsider := h.account(t, "runtime-outside", "projects:read", "projects:write")
	project := rowID(t, h.ok(t, owner, "project.create", map[string]any{"code": "R1", "name": "Runtime"}))
	tasks := []any{}
	for i := 0; i < 25; i++ {
		tasks = append(tasks, rowID(t, h.ok(t, owner, "project.task.create", map[string]any{"projectId": project, "title": fmt.Sprintf("Task %02d", i)})))
	}
	list := h.ok(t, owner, "project.task.list", map[string]any{"projectId": project, "page": 2, "pageSize": 20})
	data := decodePayload[struct {
		Items []map[string]any `json:"items"`
		Total int              `json:"total"`
	}](t, list.Data)
	if len(data.Items) != 5 || data.Total != 25 {
		t.Fatal(data)
	}
	ids := []any{tasks[0], tasks[1], int64(9999999), tasks[0]}
	input := map[string]any{"ids": ids, "assigneeId": owner.id, "operationId": "assignment-1"}
	result := h.ok(t, owner, "project.task.assign.batch", input)
	rows := decodePayload[struct {
		Items []struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"items"`
	}](t, result.Data)
	if len(rows.Items) != 3 || rows.Items[0].Status != "succeeded" || rows.Items[1].Status != "succeeded" || rows.Items[2].Status == "succeeded" {
		t.Fatal(rows)
	}
	// A new envelope can recover an interrupted attempt with the same operation ID.
	h.ok(t, owner, "project.task.assign.batch", input)
	var events int
	if err := h.pool.QueryRow(owner.ctx, "SELECT count(*) FROM cap_outbox WHERE tenant_id='tenant-a' AND event_type='project.task.assigned'").Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 2 {
		t.Fatalf("duplicate assignment events %d", events)
	}
	result, err := h.call(t, outsider, "project.task.assign.batch", input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != contracts.StatusSucceeded {
		t.Fatal(result)
	}
	denied := decodePayload[struct {
		Items []struct {
			Status string `json:"status"`
		} `json:"items"`
	}](t, result.Data)
	for _, r := range denied.Items {
		if r.Status == "succeeded" {
			t.Fatal("nonmember assigned a task")
		}
	}
	result, err = h.call(t, owner, "project.task.assign.batch", map[string]any{"ids": []any{}, "assigneeId": owner.id, "operationId": "invalid"})
	rejected(t, result, err, "invalid")
	foreign, err := h.callWith(t, projectActor{id: owner.id, ctx: outsider.ctx}, "project.task.list", map[string]any{"projectId": project, "page": 1, "pageSize": 20}, "outside-list", contracts.OriginHumanUI)
	rejected(t, foreign, err, "not_found")
	// An actor retaining the same principal loses access immediately after membership removal.
	h.ok(t, owner, "project.member.add", map[string]any{"projectId": project, "accountId": outsider.id})
	h.ok(t, outsider, "project.task.assign.batch", map[string]any{"ids": []any{tasks[0]}, "assigneeId": owner.id, "operationId": "member-before-revoke"})
	h.ok(t, owner, "project.member.remove", map[string]any{"projectId": project, "accountId": outsider.id})
	revoked := h.ok(t, outsider, "project.task.assign.batch", map[string]any{"ids": []any{tasks[0]}, "assigneeId": owner.id, "operationId": "member-after-revoke"})
	revokedData := decodePayload[struct {
		Items []struct {
			Status string `json:"status"`
		} `json:"items"`
	}](t, revoked.Data)
	if revokedData.Items[0].Status == "succeeded" {
		t.Fatal("removed member retained batch access")
	}
	var wrongOrigins int
	if err := h.pool.QueryRow(owner.ctx, "SELECT count(*) FROM cap_invocations WHERE capability='project.task.assign' AND origin<>'capability'").Scan(&wrongOrigins); err != nil {
		t.Fatal(err)
	}
	if wrongOrigins != 0 {
		t.Fatal("nested batch origin was forged")
	}

}
