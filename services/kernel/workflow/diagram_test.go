package orchestration

import (
	"testing"

	"baseline/services/api/kernel/workflow/spec"
	"github.com/cloudwego/eino/compose"
)

func TestBuildDiagramIgnoresCompiledFanOut(t *testing.T) {
	d := LifecycleExample()
	info := &compose.GraphInfo{
		Nodes: map[string]compose.GraphNodeInfo{
			"create":  {Name: "create"},
			"wait":    {Name: "wait"},
			"review":  {Name: "review"},
			"archive": {Name: "archive"},
		},
		// A real Eino Workflow attaches the root input to every node and every
		// step to END. Treating those as control edges makes the canvas look
		// fully connected, so the projection must ignore them.
		Edges: map[string][]string{
			compose.START: {"create", "wait", "review", "archive"},
			"create":      {"wait", "review", "archive", compose.END},
			"wait":        {"review", compose.END},
			"review":      {"archive", compose.END},
			"archive":     {compose.END},
		},
		DataEdges: map[string][]string{"start": {"review", "archive"}},
	}
	got := buildDiagram(d, info)
	if len(got.Nodes) != len(d.Steps)+2 {
		t.Fatalf("expected start, end and every step, got %d nodes", len(got.Nodes))
	}
	for _, node := range got.Nodes {
		if node.Kind == "start" || node.Kind == "end" {
			continue
		}
		if !node.Compiled {
			t.Fatalf("node %s exists in the compiled graph but is marked declared-only", node.ID)
		}
	}
	want := []string{
		"archive->end",
		"create->wait",
		"review->archive",
		"start->create",
		"wait->review",
	}
	if len(got.Edges) != len(want) {
		t.Fatalf("expected the declared chain only, got %d edges: %+v", len(got.Edges), got.Edges)
	}
	for i, edge := range got.Edges {
		key := edge.From + "->" + edge.To
		if key != want[i] {
			t.Fatalf("edge %d = %s, want %s (all: %+v)", i, key, want[i], got.Edges)
		}
		if edge.Kind != "control" {
			t.Fatalf("edge %s should be control, got %s", key, edge.Kind)
		}
	}
	if got.Hash != d.Hash() {
		t.Fatal("diagram must pin the definition hash")
	}
	if got.Input == nil {
		t.Fatal("diagram must expose the input schema")
	}
}

func TestBuildDiagramMarksBranchEdges(t *testing.T) {
	d := spec.ArticlePublish()
	got := buildDiagram(d, nil)
	kinds := map[string]string{}
	for _, edge := range got.Edges {
		kinds[edge.From+"->"+edge.To] = edge.Kind
	}
	if kinds["record->publish"] != "branch" || kinds["record->reject"] != "branch" {
		t.Fatalf("conditional steps must be reached by branch edges: %+v", got.Edges)
	}
	// A conditional step is reached from its branch source only, never from START.
	for key := range kinds {
		if key == "start->publish" || key == "start->reject" {
			t.Fatalf("branch target must not also hang off START: %+v", got.Edges)
		}
	}
	if kinds["start->save"] != "control" || kinds["save->review"] != "control" {
		t.Fatalf("declared chain lost: %+v", got.Edges)
	}
	if kinds["publish->end"] != "control" || kinds["reject->end"] != "control" {
		t.Fatalf("terminal steps must close into END: %+v", got.Edges)
	}
}

func TestBuildDiagramFallsBackToDeclaredDependencies(t *testing.T) {
	d := LifecycleExample()
	got := buildDiagram(d, nil)
	for _, node := range got.Nodes {
		if node.Kind != "start" && node.Kind != "end" && node.Compiled {
			t.Fatalf("uncompiled node %s cannot claim to be compiled", node.ID)
		}
	}
	if len(got.Edges) == 0 {
		t.Fatal("declared dependency fallback produced no edges")
	}
	pairs := map[string]bool{}
	for _, edge := range got.Edges {
		pairs[edge.From+"->"+edge.To] = true
	}
	if !pairs["create->wait"] {
		t.Fatalf("input mapping dependency missing: %+v", got.Edges)
	}
	if !pairs["review->archive"] {
		t.Fatalf("declared dependency missing: %+v", got.Edges)
	}
	if !pairs["archive->"+compose.END] {
		t.Fatalf("terminal step must connect to end: %+v", got.Edges)
	}
}

func TestBuildDiagramMarksNodeKinds(t *testing.T) {
	got := buildDiagram(LifecycleExample(), nil)
	byID := map[string]string{}
	for _, node := range got.Nodes {
		byID[node.ID] = node.Kind
	}
	if byID["review"] != "agent" {
		t.Fatalf("agent step kind lost: %+v", byID)
	}
	if byID["wait"] != "poll" {
		t.Fatalf("poll step kind lost: %+v", byID)
	}
	if byID["create"] != "step" {
		t.Fatalf("capability step kind lost: %+v", byID)
	}
}
