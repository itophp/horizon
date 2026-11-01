// Package spec defines the pure workflow contract, its JSON Schema and semantic
// checks. It has no application, Goa or generated SQL dependencies.
package spec

import (
	"baseline/services/api/internal/platform/capability/contracts"
	"baseline/services/api/internal/platform/capability/registry"
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
}
type Definition struct {
	Name    string            `json:"name"`
	Version string            `json:"version"`
	Input   *contracts.Schema `json:"input"`
	Steps   []Step            `json:"steps"`
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
	if err := validateShape(d); err != nil {
		return err
	}
	if !identifier.MatchString(d.Name) || d.Version == "" || d.Input == nil || d.Input.Type != "object" || len(d.Version) > MaxVersionLength || len(d.Steps) == 0 || len(d.Steps) > MaxWorkflowSteps {
		return fmt.Errorf("invalid workflow header")
	}
	byID := map[string]Step{}
	footprints := map[string][]contracts.Definition{}
	for _, s := range d.Steps {
		if !identifier.MatchString(s.ID) || s.ID == "input" {
			return fmt.Errorf("invalid step id")
		}
		if _, ok := byID[s.ID]; ok {
			return fmt.Errorf("duplicate step %s", s.ID)
		}
		byID[s.ID] = s
		names := []string{s.Capability}
		if s.MaxAttempts < 0 || s.MaxAttempts > MaxReadAttempts {
			return fmt.Errorf("invalid retry bound")
		}
		if s.Agent != nil {
			if s.Capability != "" || s.Poll != nil || len(s.Agent.Tools) == 0 || s.Agent.MaxIterations < 1 || s.Agent.MaxIterations > MaxAgentIterations || s.Agent.Instruction == "" {
				return fmt.Errorf("invalid agent %s", s.ID)
			}
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
			if !ok || e.Definition.Layer != contracts.LayerAtomic {
				return fmt.Errorf("step %s requires an atomic capability", s.ID)
			}
			footprints[s.ID] = append(footprints[s.ID], e.Definition.Normalized())
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
		for _, dep := range deps[s.ID] {
			if _, ok := byID[dep]; !ok || dep == s.ID {
				return fmt.Errorf("unknown or self dependency on %s", s.ID)
			}
		}
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
