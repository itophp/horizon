package spec

import (
	"fmt"
	"slices"

	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/capability/registry"
)

// GraphReference pins a code-registered Eino Graph specification. It is not
// executable code supplied by a client or a second capability definition.
type GraphReference struct {
	Ref  string `json:"ref"`
	Hash string `json:"hash"`
}
type GraphRead struct {
	Capability string            `json:"capability"`
	Version    string            `json:"version"`
	Input      map[string]string `json:"input"`
}
type GraphDefinition struct {
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	Input           *contracts.Schema `json:"input"`
	Output          *contracts.Schema `json:"output"`
	Reads           []GraphRead       `json:"reads"`
	Instruction     string            `json:"instruction"`
	MaxModelCalls   int               `json:"maxModelCalls"`
	MaxToolCalls    int               `json:"maxToolCalls"`
	MaxOutputTokens int               `json:"maxOutputTokens"`
	MaxContextBytes int               `json:"maxContextBytes"`
	TimeoutMillis   int               `json:"timeoutMillis"`
}

func (g GraphDefinition) Key() string  { return g.Name + "@" + g.Version }
func (g GraphDefinition) Hash() string { return contracts.Digest(contracts.Input{"graph": g}) }
func (g GraphDefinition) Reference() *GraphReference {
	return &GraphReference{Ref: g.Key(), Hash: g.Hash()}
}
func (g GraphDefinition) Validate(r *registry.Registry) error {
	if !identifier.MatchString(g.Name) || g.Version == "" || len(g.Version) > MaxVersionLength || g.Input == nil || g.Input.Type != "object" || g.Output == nil || g.Output.Type != "object" || len(g.Output.Properties) == 0 || g.Instruction == "" || g.MaxModelCalls < 1 || g.MaxModelCalls > 8 || g.MaxToolCalls < len(g.Reads) || g.MaxToolCalls > 16 || g.MaxOutputTokens < 1 || g.MaxOutputTokens > 2048 || g.MaxContextBytes < 256 || g.MaxContextBytes > 32768 || g.TimeoutMillis < 1 || g.TimeoutMillis > 30000 {
		return fmt.Errorf("invalid graph contract or budget")
	}
	seen := map[string]bool{}
	for _, read := range g.Reads {
		entry, ok := r.Resolve(read.Capability, read.Version)
		if !ok || read.Version == "" || !GraphReadable(entry.Definition) || seen[entry.Definition.Key()] {
			return fmt.Errorf("graph requires unique pinned non-sensitive atomic reads")
		}
		seen[entry.Definition.Key()] = true
		for name, field := range read.Input {
			if _, ok := entry.Definition.Input.Properties[name]; !ok {
				return fmt.Errorf("unknown graph read argument")
			}
			if _, ok := g.Input.Properties[field]; !ok {
				return fmt.Errorf("unknown graph input field")
			}
		}
		for _, field := range entry.Definition.Input.Required {
			source, ok := read.Input[field]
			if !ok || !slices.Contains(g.Input.Required, source) {
				return fmt.Errorf("missing required graph read source")
			}
		}
	}
	return nil
}
func GraphReadable(d contracts.Definition) bool {
	return d.ReadOnly && d.Layer == contracts.LayerAtomic && !d.TransientResult && len(d.OutputAccess) == 0 && d.Approval == contracts.ApprovalAuto && d.Risk == contracts.RiskLow && d.AllowsOrigin(contracts.OriginAgent) && !slices.Contains(d.Tags, "manual-only") && !slices.Contains(d.Tags, "internal-only") && !slices.Contains(d.Tags, "system-only")
}

// GraphWorkflow is an infrastructure wrapper so standalone Graph plans reuse
// the existing durable Workflow queue and encrypted budget/checkpoint store.
func GraphWorkflow(g GraphDefinition) Definition {
	input := map[string]Mapping{}
	for _, field := range g.Input.Required {
		input[field] = Mapping{From: "input", Field: field}
	}
	return Definition{Name: "graph_" + g.Hash()[:24], Version: g.Version, FreshContext: true, Input: g.Input, Steps: []Step{{ID: "result", Graph: g.Reference(), Input: input}}}
}
