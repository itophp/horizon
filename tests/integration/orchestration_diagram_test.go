//go:build integration

package tests

import (
	"context"
	"horizon/applications/articles"
	"horizon/applications/example"
	"horizon/foundation/kernel/approval"
	"horizon/foundation/kernel/audit"
	"horizon/foundation/kernel/capability/gateway"
	"horizon/foundation/kernel/capability/registry"
	"horizon/foundation/kernel/eventhub"
	runtime "horizon/foundation/kernel/workflow"
	"horizon/foundation/system/governance"
	"horizon/foundation/system/identity"
	orchmodule "horizon/foundation/system/orchestration"
	"testing"

	"github.com/cloudwego/eino/compose"
)

// The diagram is only useful if Eino really compiled the definition, so this
// case runs the real engine against isolated PostgreSQL instead of faking a
// GraphInfo.
func TestWorkflowDiagramProjectsCompiledGraph(t *testing.T) {
	_, pool, root := iamFixture(t)
	ctx := context.Background()
	iam := identity.New(pool, false)
	core := governance.New(pool, iam, governance.Options{EncryptionKey: []byte("01234567890123456789012345678901")})
	e := runtime.New(core, runtime.Builtins())
	entries := append(example.New(pool).Entries(), (&orchmodule.Module{Engine: e}).Entries()...)
	entries = append(entries, (&articles.Module{Pool: pool}).Entries()...)
	catalog := registry.New(entries)
	if err := catalog.Err(); err != nil {
		t.Fatal(err)
	}
	g := gateway.New(gateway.Options{Registry: catalog, Identity: iam, Pool: pool, Approvals: approval.NewStore(pool), Audit: audit.NewStore(pool), Events: eventhub.NewStore(pool)})
	if err := e.Attach(g, catalog); err != nil {
		t.Fatal(err)
	}
	p, err := iam.PrincipalForTask(ctx, "tenant-a", root.ID, root.DepartmentID)
	if err != nil {
		t.Fatal(err)
	}
	ctx = identity.WithPrincipal(ctx, p)

	diagram, err := e.Diagram(ctx, "example-lifecycle", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(diagram.Nodes) != 6 {
		t.Fatalf("expected start, end and four steps, got %d nodes", len(diagram.Nodes))
	}
	if len(diagram.Edges) == 0 {
		t.Fatal("compiled graph exposed no edges")
	}
	kinds := map[string]string{}
	for _, node := range diagram.Nodes {
		kinds[node.ID] = node.Kind
		if node.Kind != "start" && node.Kind != "end" && !node.Compiled {
			t.Fatalf("node %s is missing from the compiled graph", node.ID)
		}
	}
	if kinds["review"] != "agent" || kinds["wait"] != "poll" || kinds["create"] != "step" {
		t.Fatalf("node kinds lost: %+v", kinds)
	}
	edges := map[string]bool{}
	for _, edge := range diagram.Edges {
		edges[edge.From+"->"+edge.To] = true
	}
	if !edges[compose.START+"->create"] {
		t.Fatalf("start edge missing: %+v", diagram.Edges)
	}
	if !edges["archive->"+compose.END] {
		t.Fatalf("end edge missing: %+v", diagram.Edges)
	}
	if diagram.Hash != runtime.LifecycleExample().Hash() {
		t.Fatal("diagram hash does not pin the definition")
	}
	if _, err := e.Diagram(ctx, "missing-flow", "1.0.0"); err == nil {
		t.Fatal("unknown workflow must not produce a diagram")
	}
}
