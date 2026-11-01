// Package dictionaries holds the domain types and the capability definitions of the
// dictionaries module. Types are owned by the domain, not by the transport layer.
package dictionaries

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
type DictionaryDetail struct {
	Document         *DictionaryDocument   `json:"document"`
	Drafts           []*ChangeRecord       `json:"drafts"`
	History          []*ChangeRecord       `json:"history"`
	Overrides        []*DictionaryOverride `json:"overrides"`
	References       int64                 `json:"references"`
	OverrideRevision int64                 `json:"overrideRevision"`
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
type DictionaryOption struct {
	ID         int64  `json:"id"`
	Source     string `json:"source"`
	Code       string `json:"code"`
	Label      string `json:"label"`
	ParentCode string `json:"parentCode"`
	Disabled   bool   `json:"disabled"`
	IsDefault  bool   `json:"isDefault"`
	Color      string `json:"color"`
	Icon       string `json:"icon"`
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
type DictionaryWorkspace struct {
	Target      *ConfigTarget           `json:"target"`
	Definitions []*DictionaryDefinition `json:"definitions"`
}
type DraftPayload struct {
	Document     *DictionaryDocument `json:"document"`
	BaseRevision int64               `json:"baseRevision"`
	Reason       string              `json:"reason"`
}
type ExportPayload struct {
	ID     int64  `json:"id"`
	Format string `json:"format"`
}
type ExportResult struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}
type GetPayload struct {
	ID         int64  `json:"id"`
	TargetKind string `json:"targetKind"`
	TargetID   int64  `json:"targetID"`
}
type ImportDraftPayload struct {
	ID           int64  `json:"id"`
	Format       string `json:"format"`
	Content      string `json:"content"`
	BaseRevision int64  `json:"baseRevision"`
	Reason       string `json:"reason"`
}
type ImportPayload struct {
	ID      int64  `json:"id"`
	Format  string `json:"format"`
	Content string `json:"content"`
}
type ImportPreview struct {
	Document *DictionaryDocument `json:"document"`
	Errors   []string            `json:"errors"`
	Added    int                 `json:"added"`
	Updated  int                 `json:"updated"`
}
type ListPayload struct {
	TargetKind string `json:"targetKind"`
	TargetID   int64  `json:"targetID"`
}
type OptionsPayload struct {
	Code     string  `json:"code"`
	Language string  `json:"language"`
	Ids      []int64 `json:"ids"`
	Search   string  `json:"search"`
}
type OverridePayload struct {
	ID           int64                 `json:"id"`
	Target       *ConfigTarget         `json:"target"`
	BaseRevision int64                 `json:"baseRevision"`
	Overrides    []*DictionaryOverride `json:"overrides"`
	Extensions   []*DictionaryItem     `json:"extensions"`
	Reason       string                `json:"reason"`
}
type PublishPayload struct {
	ID           int64  `json:"id"`
	BaseRevision int64  `json:"baseRevision"`
	Proof        string `json:"proof"`
}
type RemovePayload struct {
	ID int64 `json:"id"`
}
type RollbackPayload struct {
	ID           int64  `json:"id"`
	BaseRevision int64  `json:"baseRevision"`
	Proof        string `json:"proof"`
	Reason       string `json:"reason"`
}
