// Package settings holds the domain types and the capability definitions of the
// settings module. Types are owned by the domain, not by the transport layer.
package settings

type CancelSchedulePayload struct {
	ID int64 `json:"id"`
}
type ChangeEntry struct {
	Key       string `json:"key"`
	Operation string `json:"operation"`
	Value     string `json:"value"`
}
type ChangeRecord struct {
	Document     *DictionaryDocument   `json:"document"`
	Overrides    []*DictionaryOverride `json:"overrides"`
	Domain       string                `json:"domain"`
	ID           int64                 `json:"id"`
	Target       *ConfigTarget         `json:"target"`
	Revision     int64                 `json:"revision"`
	BaseRevision int64                 `json:"baseRevision"`
	Status       string                `json:"status"`
	Reason       string                `json:"reason"`
	ActorID      int64                 `json:"actorId"`
	CreatedAt    string                `json:"createdAt"`
	Sensitive    bool                  `json:"sensitive"`
	Changes      []*ChangeEntry        `json:"changes"`
}
type ConfigDefinition struct {
	Key          string   `json:"key"`
	Group        string   `json:"group"`
	Kind         string   `json:"kind"`
	DefaultValue string   `json:"defaultValue"`
	Scopes       []string `json:"scopes"`
	Choices      []string `json:"choices"`
	Sensitive    bool     `json:"sensitive"`
	Secret       bool     `json:"secret"`
}
type ConfigEntry struct {
	Key        string `json:"key"`
	Value      string `json:"value"`
	Source     string `json:"source"`
	Overridden bool   `json:"overridden"`
	Configured bool   `json:"configured"`
}
type ConfigTarget struct {
	Kind string `json:"kind"`
	ID   int64  `json:"id"`
}
type DictionaryDefinition struct {
	ID                 int64         `json:"id"`
	Code               string        `json:"code"`
	NameEn             string        `json:"nameEn"`
	NameZh             string        `json:"nameZh"`
	Category           string        `json:"category"`
	Description        string        `json:"description"`
	Kind               string        `json:"kind"`
	Provider           string        `json:"provider"`
	Target             *ConfigTarget `json:"target"`
	Status             string        `json:"status"`
	AllowExtensions    bool          `json:"allowExtensions"`
	Visibility         string        `json:"visibility"`
	ConsumerPermission string        `json:"consumerPermission"`
	Revision           int64         `json:"revision"`
}
type DictionaryDocument struct {
	Definition *DictionaryDefinition `json:"definition"`
	Items      []*DictionaryItem     `json:"items"`
}
type DictionaryItem struct {
	Owner         *ConfigTarget `json:"owner"`
	ID            int64         `json:"id"`
	Code          string        `json:"code"`
	LabelEn       string        `json:"labelEn"`
	LabelZh       string        `json:"labelZh"`
	DescriptionEn string        `json:"descriptionEn"`
	DescriptionZh string        `json:"descriptionZh"`
	ParentCode    string        `json:"parentCode"`
	Sort          int           `json:"sort"`
	Status        string        `json:"status"`
	IsDefault     bool          `json:"isDefault"`
	StartsAt      string        `json:"startsAt"`
	EndsAt        string        `json:"endsAt"`
	Color         string        `json:"color"`
	Icon          string        `json:"icon"`
}
type DictionaryOverride struct {
	ItemID    int64    `json:"itemId"`
	Reset     bool     `json:"reset"`
	Fields    []string `json:"fields"`
	LabelEn   string   `json:"labelEn"`
	LabelZh   string   `json:"labelZh"`
	Sort      int      `json:"sort"`
	Status    string   `json:"status"`
	IsDefault bool     `json:"isDefault"`
}
type DraftPayload struct {
	Target       *ConfigTarget  `json:"target"`
	BaseRevision int64          `json:"baseRevision"`
	Reason       string         `json:"reason"`
	Changes      []*ChangeEntry `json:"changes"`
}
type EffectiveConfiguration struct {
	Entries  []*ConfigEntry `json:"entries"`
	Revision string         `json:"revision"`
}
type EffectivePayload struct {
}
type ExportPayload struct {
	TargetKind string `json:"targetKind"`
	TargetID   int64  `json:"targetID"`
}
type FileContent struct {
	Name    string `json:"name"`
	Mime    string `json:"mime"`
	Content []byte `json:"content"`
}
type LogoPayload struct {
}
type PreferencesPayload struct {
	Changes []*ChangeEntry `json:"changes"`
}
type PublishPayload struct {
	ID           int64  `json:"id"`
	BaseRevision int64  `json:"baseRevision"`
	Proof        string `json:"proof"`
}
type ReauthProof struct {
	Proof     string `json:"proof"`
	ExpiresAt string `json:"expiresAt"`
}
type ReauthenticatePayload struct {
	Password   string `json:"password"`
	Operation  string `json:"operation"`
	ResourceID int64  `json:"resourceId"`
	Revision   int64  `json:"revision"`
}
type RollbackPayload struct {
	ID           int64  `json:"id"`
	BaseRevision int64  `json:"baseRevision"`
	Proof        string `json:"proof"`
	Reason       string `json:"reason"`
}
type SchedulePayload struct {
	ID           int64  `json:"id"`
	BaseRevision int64  `json:"baseRevision"`
	Proof        string `json:"proof"`
	ScheduledAt  string `json:"scheduledAt"`
}
type SchedulesPayload struct {
}
type SettingSchedule struct {
	ID           int64  `json:"id"`
	DraftID      int64  `json:"draftId"`
	ActorID      int64  `json:"actorId"`
	DepartmentID int64  `json:"departmentId"`
	BaseRevision int64  `json:"baseRevision"`
	Reason       string `json:"reason"`
	ScheduledAt  string `json:"scheduledAt"`
	CreatedAt    string `json:"createdAt"`
	FinishedAt   string `json:"finishedAt"`
	Status       string `json:"status"`
	ErrorCode    string `json:"errorCode"`
	VersionID    int64  `json:"versionId"`
}
type SettingsWorkspace struct {
	Target      *ConfigTarget       `json:"target"`
	Revision    int64               `json:"revision"`
	Definitions []*ConfigDefinition `json:"definitions"`
	Entries     []*ConfigEntry      `json:"entries"`
	Drafts      []*ChangeRecord     `json:"drafts"`
	History     []*ChangeRecord     `json:"history"`
}
type WorkspacePayload struct {
	TargetKind string `json:"targetKind"`
	TargetID   int64  `json:"targetID"`
}
