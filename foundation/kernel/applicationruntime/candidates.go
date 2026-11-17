package applicationruntime

import "horizon/foundation/kernel/capability/contracts"

func DeploymentCandidates(applications []ApplicationV2) []Candidate {
	out := []Candidate{}
	labels := map[int]map[string]string{}
	for _, app := range applications {
		found := -1
		for i, c := range out {
			if c.PluginID == app.Source && c.Version == app.PluginVersion {
				found = i
			}
		}
		if found < 0 {
			out = append(out, Candidate{PluginID: app.Source, Version: app.PluginVersion, Applications: []string{}, Dependencies: []string{}, DependencyVersions: app.PluginDependencies})
			found = len(out) - 1
		}
		out[found].Applications = append(out[found].Applications, app.ID)
		if app.LabelKey != "" {
			if labels[found] == nil {
				labels[found] = map[string]string{}
			}
			labels[found][app.ID] = app.LabelKey
		}
		for _, dependency := range app.Dependencies {
			for _, other := range applications {
				if other.ID == dependency {
					out[found].Dependencies = append(out[found].Dependencies, other.Source)
				}
			}
		}
		out[found].Digest = contracts.Digest(contracts.Input{"apps": out[found].Applications, "metadata": app.MetadataVersion, "previous": out[found].Digest})
	}
	for i := range out {
		if labels[i] != nil {
			out[i].LabelKeys = labels[i]
		}
	}
	return Canonical(out)
}

// MarkBase records the platform-managed classification on matching packages.
// The base set is deployment composition data, never tenant input.
func MarkBase(candidates []Candidate, base map[string]bool) []Candidate {
	for i := range candidates {
		candidates[i].Base = base[candidates[i].PluginID]
	}
	return candidates
}
