package applicationruntime

type Installation struct {
	PluginID        string `json:"pluginId"`
	Version         string `json:"version"`
	Status          string `json:"status"`
	Revision        int64  `json:"revision"`
	Digest          string `json:"digest"`
	PreviousVersion string `json:"previousVersion"`
}
type Candidate struct {
	DependencyVersions []PluginDependency `json:"dependencyVersions"`
	PluginID           string             `json:"pluginId"`
	Version            string             `json:"version"`
	Digest             string             `json:"digest"`
	Applications       []string           `json:"applications"`
	Dependencies       []string           `json:"dependencies"`
}
type ApplicationState struct {
	ApplicationID string `json:"applicationId"`
	PluginID      string `json:"pluginId"`
	Enabled       bool   `json:"enabled"`
	Revision      int64  `json:"revision"`
}
type InstallationCatalog struct {
	Applications []ApplicationState `json:"applications"`
	Items        []Installation     `json:"items"`
	Candidates   []Candidate        `json:"candidates"`
}
type CompatibilityReport struct {
	Compatible bool      `json:"compatible"`
	Issues     []string  `json:"issues"`
	Candidate  Candidate `json:"candidate"`
}
