package applicationruntime

import "fmt"

// V2 contracts are independent of the business entity schema. Expressions are
// data, never executable JavaScript. Literal holds JSON, validated at assembly.
type Expression struct {
	Kind    string       `json:"kind"`
	Path    string       `json:"path"`
	Literal string       `json:"literal"`
	Items   []Expression `json:"items"`
	Fields  []Mapping    `json:"fields"`
}
type Mapping struct {
	Target string     `json:"target"`
	Value  Expression `json:"value"`
}
type CapabilityRef struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
type Provider struct {
	FilterTarget  string         `json:"filterTarget,omitempty"`
	TimeoutMs     int            `json:"timeoutMs,omitempty"`
	StaleTimeMs   int            `json:"staleTimeMs,omitempty"`
	Select        []string       `json:"select,omitempty"`
	QueryBindings []QueryBinding `json:"queryBindings,omitempty"`
	MaxDepth      int            `json:"maxDepth,omitempty"`
	Adapter       string         `json:"adapter"`
	OutputSchema  string         `json:"outputSchema"`
	Permission    string         `json:"permission"`
	InputSchema   string         `json:"inputSchema"`
	ID            string         `json:"id"`
	Capability    CapabilityRef  `json:"capability"`
	Input         []Mapping      `json:"input"`
	ResultPath    string         `json:"resultPath"`
	KeyField      string         `json:"keyField"`
	LabelField    string         `json:"labelField"`
	Pagination    string         `json:"pagination"`
	PageSize      int            `json:"pageSize"`
	PollInterval  int            `json:"pollInterval"`
	Tags          []string       `json:"tags"`
}
type FieldV2 struct {
	Semantic         bool       `json:"semantic,omitempty"`
	MinItems         int        `json:"minItems,omitempty"`
	Permission       string     `json:"permission,omitempty"`
	WritePermission  string     `json:"writePermission,omitempty"`
	DisabledWhen     Expression `json:"disabledWhen,omitempty"`
	Compute          Expression `json:"compute,omitempty"`
	DependsOn        []string   `json:"dependsOn,omitempty"`
	Children         []FieldV2  `json:"children,omitempty"`
	MaxItems         int        `json:"maxItems,omitempty"`
	Multiple         bool       `json:"multiple,omitempty"`
	SearchParam      string     `json:"searchParam,omitempty"`
	Group            string     `json:"group,omitempty"`
	Timezone         string     `json:"timezone,omitempty"`
	Name             string     `json:"name"`
	LabelKey         string     `json:"labelKey"`
	Type             string     `json:"type"`
	Component        string     `json:"component"`
	ComponentVersion string     `json:"componentVersion"`
	Required         bool       `json:"required"`
	ReadOnly         bool       `json:"readOnly"`
	Sensitive        bool       `json:"sensitive"`
	Derived          bool       `json:"derived"`
	Hidden           bool       `json:"hidden"`
	VisibleWhen      Expression `json:"visibleWhen"`
	MinLength        int        `json:"minLength"`
	MaxLength        int        `json:"maxLength"`
	Minimum          *float64   `json:"minimum,omitempty"`
	Maximum          *float64   `json:"maximum,omitempty"`
	Enum             []string   `json:"enum"`
	ValueLabelPrefix string     `json:"valueLabelPrefix"`
	Default          Expression `json:"default"`
	Reference        string     `json:"reference"`
	LinkPage         string     `json:"linkPage"`
	Width            int        `json:"width"`
	Fixed            bool       `json:"fixed"`
}
type Command struct {
	ActionType  string        `json:"actionType"`
	InputSchema string        `json:"inputSchema"`
	Capability  CapabilityRef `json:"capability"`
	Permission  string        `json:"permission"`
	Fields      []FieldV2     `json:"fields"`
	Input       []Mapping     `json:"input"`
	When        Expression    `json:"when"`
}
type FormV2 struct {
	Groups   []FormGroup `json:"groups,omitempty"`
	Draft    bool        `json:"draft,omitempty"`
	Provider string      `json:"provider"`
	ID       string      `json:"id"`
	LabelKey string      `json:"labelKey"`
	Create   Command     `json:"create"`
	Edit     Command     `json:"edit"`
	Tags     []string    `json:"tags"`
}
type Filter struct {
	Name             string   `json:"name"`
	LabelKey         string   `json:"labelKey"`
	Type             string   `json:"type"`
	Enum             []string `json:"enum"`
	ValueLabelPrefix string   `json:"valueLabelPrefix"`
}
type Sort struct {
	Value    string `json:"value"`
	LabelKey string `json:"labelKey"`
}
type TableV2 struct {
	RowPage     string    `json:"rowPage,omitempty"`
	Columns     []FieldV2 `json:"columns"`
	Filters     []Filter  `json:"filters"`
	Sorts       []Sort    `json:"sorts"`
	DefaultSort string    `json:"defaultSort"`
	Selectable  bool      `json:"selectable"`
}
type LayoutV2 struct {
	Type     string    `json:"type"`
	Columns  int       `json:"columns"`
	Sections []Section `json:"sections"`
}
type Section struct {
	PageID   string   `json:"pageId,omitempty"`
	ID       string   `json:"id"`
	LabelKey string   `json:"labelKey"`
	Fields   []string `json:"fields"`
	Slot     string   `json:"slot"`
}
type Action struct {
	Presentation string        `json:"presentation,omitempty"`
	WebMCP       bool          `json:"webMcp,omitempty"`
	EventSchema  string        `json:"eventSchema"`
	InputSchema  string        `json:"inputSchema"`
	ID           string        `json:"id"`
	Type         string        `json:"type"`
	LabelKey     string        `json:"labelKey"`
	Placement    string        `json:"placement"`
	Capability   CapabilityRef `json:"capability"`
	Permission   string        `json:"permission"`
	Input        []Mapping     `json:"input"`
	When         Expression    `json:"when"`
	ConfirmKey   string        `json:"confirmKey"`
	SuccessKey   string        `json:"successKey"`
	Tags         []string      `json:"tags"`
	Target       string        `json:"target"`
	Mode         string        `json:"mode"`
}
type PageV2 struct {
	RelationBindings []RelatedBinding `json:"relationBindings,omitempty"`
	View             string           `json:"view"`
	SchemaVersion    string           `json:"schemaVersion"`
	ID               string           `json:"id"`
	Type             string           `json:"type"`
	Mode             string           `json:"mode"`
	Route            string           `json:"route"`
	TitleKey         string           `json:"titleKey"`
	Permission       string           `json:"permission"`
	EntityRef        string           `json:"entityRef"`
	Provider         string           `json:"provider"`
	Layout           LayoutV2         `json:"layout"`
	Fields           []FieldV2        `json:"fields"`
	Table            TableV2          `json:"table"`
	Forms            []FormV2         `json:"forms"`
	Actions          []Action         `json:"actions"`
	Relations        []Relation       `json:"relations"`
}
type ExtensionV2 struct {
	Slot                 string   `json:"slot,omitempty"`
	ID                   string   `json:"id"`
	Source               string   `json:"source"`
	Version              string   `json:"version"`
	Type                 string   `json:"type"`
	Entry                string   `json:"entry"`
	RuntimeCompatibility string   `json:"runtimeCompatibility"`
	Permissions          []string `json:"permissions"`
	Dependencies         []string `json:"dependencies"`
}
type ApplicationV2 struct {
	CatalogRevision      int64                 `json:"catalogRevision,omitempty"`
	PreviewVersion       string                `json:"previewVersion,omitempty"`
	ApplicationVersion   string                `json:"applicationVersion,omitempty"`
	DefinitionDigest     string                `json:"definitionDigest,omitempty"`
	DefinitionRevision   int64                 `json:"definitionRevision,omitempty"`
	Messages             []LocaleMessage       `json:"messages,omitempty"`
	Theme                []ThemeToken          `json:"theme,omitempty"`
	PluginVersion        string                `json:"pluginVersion,omitempty"`
	PluginDependencies   []PluginDependency    `json:"pluginDependencies,omitempty"`
	AgentTools           []CapabilityRef       `json:"agentTools,omitempty"`
	ApplicationRevision  int64                 `json:"applicationRevision,omitempty"`
	InstallationRevision int64                 `json:"installationRevision,omitempty"`
	Navigation           []Navigation          `json:"navigation,omitempty"`
	Components           []ComponentDefinition `json:"components,omitempty"`
	SchemaVersion        string                `json:"schemaVersion"`
	ID                   string                `json:"id"`
	Version              string                `json:"version"`
	MetadataVersion      string                `json:"metadataVersion"`
	Source               string                `json:"source"`
	RuntimeCompatibility string                `json:"runtimeCompatibility"`
	LabelKey             string                `json:"labelKey"`
	Permission           string                `json:"permission"`
	Route                string                `json:"route"`
	Dependencies         []string              `json:"dependencies"`
	Pages                []PageV2              `json:"pages"`
	Providers            []Provider            `json:"providers"`
	Extensions           []ExtensionV2         `json:"extensions"`
}

func Ref(name string) CapabilityRef    { return CapabilityRef{Name: name, Version: "1.0.0"} }
func Path(path string) Expression      { return Expression{Kind: "path", Path: NormalizePath(path)} }
func Literal(raw string) Expression    { return Expression{Kind: "literal", Literal: raw} }
func Bind(target, path string) Mapping { return Mapping{Target: target, Value: Path(path)} }
func Const(target, raw string) Mapping { return Mapping{Target: target, Value: Literal(raw)} }
func Condition(kind string, items ...Expression) Expression {
	return Expression{Kind: kind, Items: items}
}

// SemanticContext contains authorized references, never business bodies or drafts.
type RecordReference struct {
	EntityRef string `json:"entityRef"`
	RecordID  string `json:"recordId"`
}
type SemanticAction struct {
	ID         string        `json:"id"`
	LabelKey   string        `json:"labelKey"`
	Capability CapabilityRef `json:"capability"`
}
type SemanticField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type SemanticContext struct {
	CapturedAt         string            `json:"capturedAt,omitempty"`
	ContextVersion     string            `json:"contextVersion,omitempty"`
	Revision           string            `json:"revision,omitempty"`
	Truncated          bool              `json:"truncated,omitempty"`
	Fields             []SemanticField   `json:"fields,omitempty"`
	ApplicationID      string            `json:"applicationId"`
	ApplicationVersion string            `json:"applicationVersion"`
	MetadataVersion    string            `json:"metadataVersion"`
	PageID             string            `json:"pageId"`
	PageType           string            `json:"pageType"`
	EntityRef          string            `json:"entityRef"`
	RecordID           string            `json:"recordId"`
	Selection          []RecordReference `json:"selection"`
	References         []RecordReference `json:"references"`
	AvailableActions   []SemanticAction  `json:"availableActions"`
	Locale             string            `json:"locale"`
	Timezone           string            `json:"timezone"`
	Source             string            `json:"source"`
}

type QueryBinding struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Target   string `json:"target"`
}
type FormGroup struct {
	ID       string `json:"id"`
	LabelKey string `json:"labelKey"`
	Type     string `json:"type"`
}
type Navigation struct {
	PageID   string `json:"pageId"`
	LabelKey string `json:"labelKey"`
	Icon     string `json:"icon"`
}
type ComponentDefinition struct {
	Name                string          `json:"name"`
	Version             string          `json:"version"`
	Source              string          `json:"source"`
	PropsSchema         string          `json:"propsSchema"`
	Slots               []string        `json:"slots"`
	Capabilities        []CapabilityRef `json:"capabilities"`
	Permissions         []string        `json:"permissions"`
	SemanticDescription string          `json:"semanticDescription"`
}

// NormalizeV21 is an in-memory upgrade; entity and database schemas are unchanged.
func NormalizeV21(app ApplicationV2) (ApplicationV2, error) {
	if app.SchemaVersion != "2.0" && app.SchemaVersion != "2.1" && app.SchemaVersion != "2.2" {
		return app, fmt.Errorf("schema_version_unsupported")
	}
	if app.RuntimeCompatibility != "2.0" && app.RuntimeCompatibility != "2.1" && app.RuntimeCompatibility != "2.2" {
		return app, fmt.Errorf("plugin_incompatible")
	}
	app = Canonical(app)
	if app.PluginVersion == "" {
		app.PluginVersion = app.Version
	}
	app.SchemaVersion = "2.2"
	app.RuntimeCompatibility = "2.2"
	for i := range app.Pages {
		if app.Pages[i].SchemaVersion != "2.0" && app.Pages[i].SchemaVersion != "2.1" && app.Pages[i].SchemaVersion != "2.2" {
			return app, fmt.Errorf("schema_version_unsupported")
		}
		app.Pages[i].SchemaVersion = "2.2"
	}
	return app, nil
}

type PluginDependency struct {
	ID      string `json:"id"`
	Version string `json:"version"`
}

type RelatedBinding struct {
	EntityRef  string `json:"entityRef"`
	ProviderID string `json:"providerId"`
	MaxItems   int    `json:"maxItems"`
}

type LocaleMessage struct {
	Key  string `json:"key"`
	En   string `json:"en"`
	ZhCN string `json:"zhCN"`
}
type ThemeToken struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
