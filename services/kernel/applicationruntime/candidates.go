package applicationruntime

import "baseline/services/api/kernel/capability/contracts"

func DeploymentCandidates(applications []ApplicationV2) []Candidate {
	out := []Candidate{}
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
		for _, dependency := range app.Dependencies {
			for _, other := range applications {
				if other.ID == dependency {
					out[found].Dependencies = append(out[found].Dependencies, other.Source)
				}
			}
		}
		out[found].Digest = contracts.Digest(contracts.Input{"apps": out[found].Applications, "metadata": app.MetadataVersion, "previous": out[found].Digest})
	}
	return Canonical(out)
}
