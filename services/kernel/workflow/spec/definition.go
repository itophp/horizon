// Package spec defines the pure workflow contract, its JSON Schema and semantic
// checks. It has no application, Goa or generated SQL dependencies.
package spec

import (
	"baseline/services/api/kernel/capability/contracts"
	"baseline/services/api/kernel/capability/registry"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

type Mapping struct {
	From  string `json:"from"`
	Field string `json:"field"`
}
type Step struct {
	Graph        *GraphReference    `json:"graph,omitempty"`
	CallMode     string             `json:"callMode,omitempty"`
	PlanID       string             `json:"planId,omitempty"`
	PlanVersion  int64              `json:"planVersion,omitempty"`
	ID           string             `json:"id"`
	Capability   string             `json:"capability,omitempty"`
	Version      string             `json:"version,omitempty"`
	DependsOn    []string           `json:"dependsOn,omitempty"`
	Input        map[string]Mapping `json:"input,omitempty"`
	Constants    map[string]any     `json:"constants,omitempty"`
	MaxAttempts  int                `json:"maxAttempts,omitempty"`
	Compensation *Compensation      `json:"compensation,omitempty"`
	Poll         *Poll              `json:"poll,omitempty"`
	Agent        *Agent             `json:"agent,omitempty"`
	When         *When              `json:"when,omitempty"`
}

// When makes a step optional: it runs only when an earlier step reports the
// declared value at Field. Steps sharing From and Field form one mutually
// exclusive branch group, so exactly one of them can ever execute.
type When struct {
	From   string `json:"from"`
	Field  string `json:"field"`
	Equals any    `json:"equals"`
}
type Compensation struct {
	Capability string             `json:"capability"`
	Version    string             `json:"version"`
	Input      map[string]Mapping `json:"input,omitempty"`
	Constants  map[string]any     `json:"constants,omitempty"`
}
type Poll struct {
	DoneField string `json:"doneField"`
	MaxSteps  int    `json:"maxSteps"`
}
type Agent struct {
	Instruction   string   `json:"instruction"`
	Tools         []string `json:"tools"`
	MaxIterations int      `json:"maxIterations"`
	// Output declares the structured result the node must return. Without it an
	// agent step only produces prose, which no branch condition can safely use.
	Output *contracts.Schema `json:"output,omitempty"`
}
type Definition struct {
	FreshContext bool              `json:"freshContext,omitempty"`
	ResultStep   string            `json:"resultStep,omitempty"`
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Input        *contracts.Schema `json:"input"`
	Steps        []Step            `json:"steps"`
}

const (
	IdentifierPattern  = `^[a-zA-Z][a-zA-Z0-9_-]{0,63}$`
	MaxWorkflowSteps   = 64
	MaxReadAttempts    = 5
	MaxPollSteps       = 100
	MaxAgentIterations = 20
	MaxVersionLength   = 16
	MaxDefinitionBytes = 1 << 20
)

var identifier = regexp.MustCompile(IdentifierPattern)

func (d Definition) Hash() string { return contracts.Digest(contracts.Input{"definition": d}) }

// Validate refuses malformed declarations and ambiguous business ordering.
func (d Definition) Validate(r *registry.Registry) error {
	return d.ValidateWithGraphs(r, BuiltinGraphs())
}
func (d Definition) ValidateWithGraphs(r *registry.Registry, graphs []GraphDefinition) error {
	if err := validateShape(d); err != nil {
		return err
	}
	if !identifier.MatchString(d.Name) || d.Version == "" || d.Input == nil || d.Input.Type != "object" || len(d.Version) > MaxVersionLength || len(d.Steps) == 0 || len(d.Steps) > MaxWorkflowSteps {
		return fmt.Errorf("invalid workflow header")
	}
	byID := map[string]Step{}
	for _, s := range d.Steps {
		if !identifier.MatchString(s.ID) || s.ID == "input" {
			return fmt.Errorf("invalid step id")
		}
		if _, ok := byID[s.ID]; ok {
			return fmt.Errorf("duplicate step %s", s.ID)
		}
		switch s.CallMode {
		case "", "resolve_active_plan", "invoke_native":
			if s.PlanID != "" || s.PlanVersion != 0 {
				return fmt.Errorf("unexpected plan selector")
			}
		case "invoke_pinned_plan":
			if s.PlanID == "" || s.PlanVersion < 1 {
				return fmt.Errorf("missing pinned plan")
			}
		default:
			return fmt.Errorf("unsupported call mode")
		}
		if s.Agent != nil && s.CallMode != "" {
			return fmt.Errorf("agent call mode unsupported")
		}
		byID[s.ID] = s
	}
	if d.ResultStep != "" {
		step, ok := byID[d.ResultStep]
		if !ok || step.Agent == nil || step.Agent.Output != nil {
			return fmt.Errorf("result step must be a prose agent node")
		}
	}
	footprints := map[string][]contracts.Definition{}
	outputs := map[string]*contracts.Schema{}
	for _, s := range d.Steps {
		if s.Graph != nil {
			if s.Capability != "" || s.Version != "" || s.Agent != nil || s.Poll != nil || s.MaxAttempts != 0 || s.Compensation != nil || s.CallMode != "" || s.PlanID != "" || s.PlanVersion != 0 {
				return fmt.Errorf("graph node cannot declare another executor")
			}
			var found *GraphDefinition
			for _, g := range graphs {
				if g.Key() == s.Graph.Ref {
					copy := g
					found = &copy
					break
				}
			}
			if found == nil || found.Hash() != s.Graph.Hash {
				return fmt.Errorf("unknown or changed graph reference")
			}
			if err := found.Validate(r); err != nil {
				return err
			}
			outputs[s.ID] = found.Output
			for _, read := range found.Reads {
				entry, _ := r.Resolve(read.Capability, read.Version)
				footprints[s.ID] = append(footprints[s.ID], entry.Definition.Normalized())
			}
			for _, field := range found.Input.Required {
				_, mapped := s.Input[field]
				_, constant := s.Constants[field]
				if !mapped && !constant {
					return fmt.Errorf("missing graph input")
				}
			}
			for field := range s.Input {
				if _, ok := found.Input.Properties[field]; !ok {
					return fmt.Errorf("unknown graph argument")
				}
			}
			for field := range s.Constants {
				if _, ok := found.Input.Properties[field]; !ok {
					return fmt.Errorf("unknown graph constant")
				}
			}
			continue
		}
		names := []string{s.Capability}
		if s.MaxAttempts < 0 || s.MaxAttempts > MaxReadAttempts {
			return fmt.Errorf("invalid retry bound")
		}
		if s.Agent != nil {
			if s.Capability != "" || s.Poll != nil || len(s.Agent.Tools) == 0 || s.Agent.MaxIterations < 1 || s.Agent.MaxIterations > MaxAgentIterations || s.Agent.Instruction == "" {
				return fmt.Errorf("invalid agent %s", s.ID)
			}
			if s.Agent.Output != nil && (s.Agent.Output.Type != "object" || len(s.Agent.Output.Properties) == 0) {
				return fmt.Errorf("agent %s output must be an object schema", s.ID)
			}
			outputs[s.ID] = s.Agent.Output
			names = s.Agent.Tools
		} else if s.Version == "" {
			return fmt.Errorf("step %s requires pinned capability version", s.ID)
		}
		for _, name := range names {
			version := s.Version
			if s.Agent != nil {
				var pinned bool
				name, version, pinned = strings.Cut(name, "@")
				if !pinned || version == "" {
					return fmt.Errorf("agent tools require pinned versions")
				}
			}
			e, ok := r.Resolve(name, version)
			readComposition := ok && s.Agent == nil && e.Definition.Layer == contracts.LayerOrchestration && e.Definition.ReadOnly && s.Poll == nil && s.MaxAttempts <= 1 && s.Compensation == nil
			if !ok || e.Definition.Layer != contracts.LayerAtomic && !readComposition {
				return fmt.Errorf("step %s requires an atomic capability or a single read composition", s.ID)
			}
			footprints[s.ID] = append(footprints[s.ID], e.Definition.Normalized())
			if s.Agent == nil {
				outputs[s.ID] = e.Definition.Output
			}
			if s.MaxAttempts > 1 && (!e.Definition.ReadOnly || s.Poll != nil || s.Agent != nil) {
				return fmt.Errorf("automatic retries require a read-only atomic step")
			}
			if s.Compensation != nil {
				if s.Agent != nil || s.Poll != nil || e.Definition.Normalized().Effect != contracts.EffectCompensable {
					return fmt.Errorf("compensation requires compensable atomic step")
				}
				c, ok := r.Resolve(s.Compensation.Capability, s.Compensation.Version)
				if !ok || s.Compensation.Version == "" || c.Definition.Layer != contracts.LayerAtomic {
					return fmt.Errorf("invalid compensation capability")
				}
				for _, m := range s.Compensation.Input {
					if m.From != "input" && m.From != s.ID {
						return fmt.Errorf("compensation may reference only input and its own forward result")
					}
				}
			}
			if s.Poll != nil && (!e.Definition.ReadOnly || s.Poll.MaxSteps < 1 || s.Poll.MaxSteps > MaxPollSteps || s.Poll.DoneField == "") {
				return fmt.Errorf("poll requires bounded read-only capability")
			}
		}
	}
	deps := map[string][]string{}
	for _, s := range d.Steps {
		deps[s.ID] = slices.Clone(s.DependsOn)
		for _, m := range s.Input {
			if m.From != "input" {
				deps[s.ID] = append(deps[s.ID], m.From)
			}
		}
		if s.When != nil {
			deps[s.ID] = append(deps[s.ID], s.When.From)
		}
		for _, dep := range deps[s.ID] {
			if _, ok := byID[dep]; !ok || dep == s.ID {
				return fmt.Errorf("unknown or self dependency on %s", s.ID)
			}
		}
	}
	if err := validateBranches(d.Steps, byID, outputs); err != nil {
		return err
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if visiting[id] {
			return false
		}
		if done[id] {
			return true
		}
		visiting[id] = true
		for _, dep := range deps[id] {
			if !visit(dep) {
				return false
			}
		}
		visiting[id] = false
		done[id] = true
		return true
	}
	for id := range byID {
		if !visit(id) {
			return fmt.Errorf("workflow dependency cycle")
		}
	}
	var precedes func(string, string) bool
	precedes = func(a, b string) bool {
		for _, dep := range deps[b] {
			if dep == a || precedes(a, dep) {
				return true
			}
		}
		return false
	}
	for i, a := range d.Steps {
		for _, b := range d.Steps[i+1:] {
			// Alternatives of one branch group can never both run, so ordering
			// them would invent a sequence the flow does not have.
			if exclusive(a, b) {
				continue
			}
			conflict := false
			for _, x := range footprints[a.ID] {
				for _, y := range footprints[b.ID] {
					if intersects(x.Writes, y.Reads) || intersects(x.Writes, y.Writes) || intersects(y.Writes, x.Reads) {
						conflict = true
					}
				}
			}
			if conflict && !precedes(a.ID, b.ID) && !precedes(b.ID, a.ID) {
				return fmt.Errorf("unordered resource conflict: %s / %s", a.ID, b.ID)
			}
		}
	}
	return nil
}
func intersects(a, b []string) bool {
	for _, x := range a {
		if slices.Contains(b, x) {
			return true
		}
	}
	return false
}

// BranchGroup is one mutually exclusive choice. Cases run in declaration order
// and the first match wins.
type BranchGroup struct {
	From  string
	Field string
	Cases []BranchCase
}

type BranchCase struct {
	Step   string
	Equals any
}

// Branches groups steps by their When source. Declaration order is preserved so
// the compiled branch and the validator agree on which case wins.
func (d Definition) Branches() []BranchGroup {
	out := []BranchGroup{}
	index := map[string]int{}
	for _, step := range d.Steps {
		if step.When == nil {
			continue
		}
		key := step.When.From + "\x00" + step.When.Field
		at, ok := index[key]
		if !ok {
			at = len(out)
			index[key] = at
			out = append(out, BranchGroup{From: step.When.From, Field: step.When.Field})
		}
		out[at].Cases = append(out[at].Cases, BranchCase{Step: step.ID, Equals: step.When.Equals})
	}
	return out
}

// exclusive reports whether two steps are alternatives of the same branch, in
// which case only one of them can ever execute.
func exclusive(a, b Step) bool {
	return a.When != nil && b.When != nil && a.When.From == b.When.From && a.When.Field == b.When.Field
}

func validateBranches(steps []Step, byID map[string]Step, outputs map[string]*contracts.Schema) error {
	seen := map[string]map[string]string{}
	for _, step := range steps {
		if step.When == nil {
			continue
		}
		if step.When.From == "" || step.When.Field == "" {
			return fmt.Errorf("step %s requires when.from and when.field", step.ID)
		}
		if _, ok := byID[step.When.From]; !ok {
			return fmt.Errorf("step %s branches on unknown step %s", step.ID, step.When.From)
		}
		if step.When.Equals == nil {
			return fmt.Errorf("step %s requires a branch case value", step.ID)
		}
		canonical, err := json.Marshal(step.When.Equals)
		if err != nil {
			return fmt.Errorf("step %s has an unsupported branch case value", step.ID)
		}
		key := step.When.From + "\x00" + step.When.Field
		if seen[key] == nil {
			seen[key] = map[string]string{}
		}
		if other, ok := seen[key][string(canonical)]; ok {
			return fmt.Errorf("steps %s and %s declare the same branch case", other, step.ID)
		}
		seen[key][string(canonical)] = step.ID
		if source := byID[step.When.From]; source.Agent != nil && source.Agent.Output == nil {
			return fmt.Errorf("step %s branches on an agent step without declared output", step.ID)
		}
		output := outputs[step.When.From]
		if output == nil || len(output.Properties) == 0 {
			continue
		}
		property, ok := output.Properties[step.When.Field]
		if !ok {
			return fmt.Errorf("step %s branches on undeclared field %s", step.ID, step.When.Field)
		}
		if !compatible(property.Type, step.When.Equals) {
			return fmt.Errorf("step %s branch case type does not match %s", step.ID, step.When.Field)
		}
	}
	return nil
}

// compatible keeps the authored case value and the declared field type aligned,
// so a typo fails at registration instead of silently never matching. It reads
// the JSON form because either side may come from Go literals or decoded JSON.
func compatible(kind string, value any) bool {
	raw, err := json.Marshal(value)
	if err != nil {
		return false
	}
	switch kind {
	case "boolean":
		return string(raw) == "true" || string(raw) == "false"
	case "string":
		return len(raw) > 0 && raw[0] == '"'
	case "integer", "number":
		var number float64
		return json.Unmarshal(raw, &number) == nil
	}
	return true
}
