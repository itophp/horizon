package orchestration

import (
	"context"
	"slices"
	"sort"

	"github.com/cloudwego/eino/compose"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/identity"
)

// Diagram is the compiled workflow graph projected for read-only rendering.
// It carries structure only: no run inputs, checkpoints, receipts or model
// messages ever leave the engine through this view.
type Diagram struct {
	Name    string            `json:"name"`
	Version string            `json:"version"`
	Hash    string            `json:"hash"`
	Input   *contracts.Schema `json:"input"`
	Nodes   []DiagramNode     `json:"nodes"`
	Edges   []DiagramEdge     `json:"edges"`
}

// DiagramNode mirrors one Eino node. Kind is derived from the definition step so
// the console can explain what the node does without reading the whole spec.
type DiagramNode struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind"`
	Capability   string   `json:"capability,omitempty"`
	Version      string   `json:"version,omitempty"`
	DependsOn    []string `json:"dependsOn,omitempty"`
	Tools        []string `json:"tools,omitempty"`
	MaxAttempts  int      `json:"maxAttempts,omitempty"`
	Compensation string   `json:"compensation,omitempty"`
	// When marks a conditional node: it executes only when the source step
	// reports the declared value. Siblings of the same group never both run.
	When *When `json:"when,omitempty"`
	// Compiled is true when the node exists in the graph Eino actually compiled,
	// which is the only proof that the runtime and the declaration agree.
	Compiled bool `json:"compiled"`
}

// DiagramEdge is a control or data dependency between two node ids. START and
// END use the same keys Eino uses ("start" / "end").
type DiagramEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

type diagramCapture struct{ set func(*compose.GraphInfo) }

func (c diagramCapture) OnFinish(_ context.Context, info *compose.GraphInfo) {
	if c.set != nil {
		c.set(info)
	}
}

// Describe is the capability entry point: it resolves the definition, compiles
// it (cached) and returns the graph projection.
func (e *Engine) Describe(ctx context.Context, in contracts.Input) (contracts.Output, error) {
	if _, err := identity.Current(ctx); err != nil {
		return contracts.Output{}, err
	}
	name, _ := in["flow"].(string)
	version, _ := in["version"].(string)
	diagram, err := e.Diagram(ctx, name, version)
	if err != nil {
		return contracts.Output{}, err
	}
	return contracts.Output{Data: diagram}, nil
}

// Diagram returns the compiled graph of one pinned definition.
func (e *Engine) Diagram(ctx context.Context, name, version string) (*Diagram, error) {
	d, ok := e.definitions[name+"@"+version]
	if !ok {
		return nil, contracts.Problem("not_found")
	}
	_, info, err := e.compileWithGraph(ctx, d, true)
	if err != nil {
		return nil, contracts.Problem("invalid")
	}
	return buildDiagram(d, info), nil
}

func buildDiagram(d Definition, info *compose.GraphInfo) *Diagram {
	out := &Diagram{Name: d.Name, Version: d.Version, Hash: d.Hash(), Input: d.Input}
	compiled := map[string]bool{}
	if info != nil {
		for key := range info.Nodes {
			compiled[key] = true
		}
	}
	out.Nodes = append(out.Nodes, DiagramNode{ID: compose.START, Kind: "start", Compiled: true})
	for _, step := range d.Steps {
		node := DiagramNode{
			ID:          step.ID,
			Kind:        "step",
			Capability:  step.Capability,
			Version:     step.Version,
			DependsOn:   step.DependsOn,
			MaxAttempts: step.MaxAttempts,
			Compiled:    compiled[step.ID],
			When:        step.When,
		}
		switch {
		case step.Agent != nil:
			node.Kind = "agent"
			node.Capability = ""
			node.Version = ""
			node.Tools = step.Agent.Tools
		case step.Poll != nil:
			node.Kind = "poll"
		}
		if step.Compensation != nil {
			node.Compensation = step.Compensation.Capability
		}
		out.Nodes = append(out.Nodes, node)
	}
	out.Nodes = append(out.Nodes, DiagramNode{ID: compose.END, Kind: "end", Compiled: true})
	out.Edges = declaredEdges(d)
	return out
}

// declaredEdges projects the authored order. Eino's compiled GraphInfo is
// deliberately not used here: a Workflow attaches the root input to every node
// and every step to END, so its control edges describe data availability, not
// sequencing. Rendering them makes the canvas look fully connected.
//
// Precedence per step: a conditional step is reached from its branch source;
// otherwise explicit DependsOn wins; otherwise input mappings order it. Steps
// with no successor connect to END once.
func declaredEdges(d Definition) []DiagramEdge {
	out := []DiagramEdge{}
	hasSuccessor := map[string]bool{}
	for _, step := range d.Steps {
		switch {
		case step.When != nil:
			hasSuccessor[step.When.From] = true
			out = append(out, DiagramEdge{From: step.When.From, To: step.ID, Kind: "branch"})
		case len(step.DependsOn) > 0:
			for _, dep := range step.DependsOn {
				hasSuccessor[dep] = true
				out = append(out, DiagramEdge{From: dep, To: step.ID, Kind: "control"})
			}
		default:
			deps := []string{}
			for _, mapping := range step.Input {
				if mapping.From != "input" && !slices.Contains(deps, mapping.From) {
					deps = append(deps, mapping.From)
				}
			}
			if len(deps) == 0 {
				out = append(out, DiagramEdge{From: compose.START, To: step.ID, Kind: "control"})
				continue
			}
			for _, dep := range deps {
				hasSuccessor[dep] = true
				out = append(out, DiagramEdge{From: dep, To: step.ID, Kind: "control"})
			}
		}
	}
	for _, step := range d.Steps {
		if !hasSuccessor[step.ID] {
			out = append(out, DiagramEdge{From: step.ID, To: compose.END, Kind: "control"})
		}
	}
	sortEdges(out)
	return out
}

func sortEdges(edges []DiagramEdge) {
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})
}
