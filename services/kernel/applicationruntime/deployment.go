package applicationruntime

import (
	"baseline/services/api/kernel/capability/contracts"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// AssembleDeployment checks every retained version against the same authoritative
// capability/entity catalog. Only versioned presentation contributions coexist.
func AssembleDeployment(sources []ApplicationV2, entities []contracts.Entity, defs []contracts.Definition) ([]ApplicationV2, []ApplicationV2, error) {
	latest := map[string]ApplicationV2{}
	seen := map[string]bool{}
	for _, app := range sources {
		var err error
		app, err = NormalizeV21(app)
		if err != nil {
			return nil, nil, err
		}
		key := app.ID + "@" + app.PluginVersion
		if seen[key] {
			return nil, nil, fmt.Errorf("schema_invalid: duplicate application version")
		}
		seen[key] = true
		if prior, ok := latest[app.ID]; ok {
			if prior.Source != app.Source {
				return nil, nil, fmt.Errorf("schema_invalid: application source changed")
			}
			if !newer(app.PluginVersion, prior.PluginVersion) {
				continue
			}
		}
		latest[app.ID] = app
	}

	packages := map[string]ApplicationV2{}
	for _, source := range sources {
		normalized, err := NormalizeV21(source)
		if err != nil {
			return nil, nil, err
		}
		packages[normalized.Source+"@"+normalized.PluginVersion] = normalized
	}
	visiting, done := map[string]bool{}, map[string]bool{}
	var visit func(string) error
	visit = func(key string) error {
		if visiting[key] {
			return fmt.Errorf("schema_invalid: plugin dependency cycle")
		}
		if done[key] {
			return nil
		}
		visiting[key] = true
		for _, dependency := range packages[key].PluginDependencies {
			target := dependency.ID + "@" + dependency.Version
			if _, ok := packages[target]; !ok {
				return fmt.Errorf("schema_invalid: missing plugin version")
			}
			if err := visit(target); err != nil {
				return err
			}
		}
		visiting[key] = false
		done[key] = true
		return nil
	}
	for key := range packages {
		if err := visit(key); err != nil {
			return nil, nil, err
		}
	}
	base := []ApplicationV2{}
	for _, app := range latest {
		base = append(base, app)
	}
	sort.Slice(base, func(i, j int) bool { return base[i].ID < base[j].ID })
	active, err := Assemble(base, entities, defs)
	if err != nil {
		return nil, nil, err
	}
	versions := []ApplicationV2{}
	for _, source := range sources {
		candidate := Canonical(base)
		for i := range candidate {
			if candidate[i].ID == source.ID {
				candidate[i] = source
			}
		}
		built, err := Assemble(candidate, entities, defs)
		if err != nil {
			return nil, nil, err
		}
		for _, app := range built {
			if app.ID == source.ID {
				versions = append(versions, app)
			}
		}
	}
	return active, versions, nil
}
func newer(left, right string) bool {
	l, r := strings.Split(left, "."), strings.Split(right, ".")
	if len(l) != 3 || len(r) != 3 {
		return left > right
	}
	for i := range l {
		a, _ := strconv.Atoi(l[i])
		b, _ := strconv.Atoi(r[i])
		if a != b {
			return a > b
		}
	}
	return false
}
