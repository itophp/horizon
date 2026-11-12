package applicationruntime

type DefinitionView struct {
	Enabled          bool          `json:"enabled"`
	Application      ApplicationV2 `json:"application"`
	Version          string        `json:"version"`
	Digest           string        `json:"digest"`
	Revision         int64         `json:"revision"`
	PublishedVersion string        `json:"publishedVersion"`
	ProposalVersion  string        `json:"proposalVersion"`
	ReadOnly         bool          `json:"readOnly"`
}
type DefinitionSummary struct {
	LabelKey      string `json:"labelKey"`
	LabelEn       string `json:"labelEn"`
	LabelZhCN     string `json:"labelZhCN"`
	ApplicationID string `json:"applicationId"`
	Version       string `json:"version"`
	Digest        string `json:"digest"`
	Revision      int64  `json:"revision"`
}

type DefinitionValidation struct {
	Valid bool   `json:"valid"`
	Code  string `json:"code"`
}
type DefinitionList struct {
	Items []DefinitionSummary `json:"items"`
}

type DefinitionBaseline struct {
	Digest        string `json:"digest"`
	Version       string `json:"version"`
	PluginVersion string `json:"pluginVersion"`
}
type DefinitionBaselineList struct {
	Items []DefinitionBaseline `json:"items"`
}
type DefinitionHistoryItem struct {
	Version       string `json:"version"`
	ParentVersion string `json:"parentVersion"`
	Digest        string `json:"digest"`
	Source        string `json:"source"`
	CreatedAt     string `json:"createdAt"`
}
type DefinitionHistory struct {
	Items []DefinitionHistoryItem `json:"items"`
}
type DefinitionDiff struct {
	BaseVersion         string             `json:"baseVersion"`
	CandidateVersion    string             `json:"candidateVersion"`
	BaseDefinition      ApplicationV2      `json:"baseDefinition"`
	CandidateDefinition ApplicationV2      `json:"candidateDefinition"`
	Changes             []DefinitionImpact `json:"changes"`
}

type DefinitionImpact struct {
	Section string `json:"section"`
	ID      string `json:"id"`
	Change  string `json:"change"`
}

type DefinitionMergeReport struct {
	Compatible   bool     `json:"compatible"`
	TargetDigest string   `json:"targetDigest"`
	Conflicts    []string `json:"conflicts"`
}
