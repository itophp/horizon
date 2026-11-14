package applicationruntime

import (
	"encoding/json"
	"fmt"
	"horizon/foundation/kernel/capability/contracts"
	"regexp"
	"slices"
	"strings"
)

// ============================================================================
// ApplicationRuntime Schema Protocol v3
//
// Three node categories:
//   1. Layout  — spatial arrangement (CSS Grid), can nest other layouts
//   2. Data    — data sources scoped to a layout, referenced as "$id"
//   3. Component — pre-built UI components with configurable props and optional content
//
// Nesting rules:
//   - A layout node may contain EITHER exactly 1 component OR 0-N layouts
//   - Container components may host a nested layout
//   - Data references "$id" must resolve to an ancestor layout's DataSpec
// ============================================================================

// Node is the discriminated union for layout and component nodes.
type Node struct {
	Type string `json:"type"` // "layout" | "component"

	// Valid when Type == "layout"
	Layout *LayoutSpec `json:"layout,omitempty"`

	// Valid when Type == "component"
	Component *ComponentSpec `json:"component,omitempty"`
}

// ============================================================================
// 1. Layout Nodes
// ============================================================================

// LayoutSpec defines a spatial container using CSS Grid or other layout modes.
type LayoutSpec struct {
	Visible  *Expression `json:"visible,omitempty"`
	ID       string      `json:"id"`
	Kind     string      `json:"kind"`               // "grid" | "stack" | "sidebar" | "tabs" | "split"
	Props    LayoutProps `json:"props"`              // Layout configuration
	Data     []DataSpec  `json:"data,omitempty"`     // Data sources scoped to this layout
	Children []Node      `json:"children,omitempty"` // Child nodes (layouts or one component)
}

// LayoutProps holds CSS Grid and general layout properties.
type LayoutProps struct {
	// CSS Grid properties
	Columns string `json:"columns,omitempty"` // grid-template-columns: "1fr 2fr" | "repeat(3, 1fr)"
	Rows    string `json:"rows,omitempty"`    // grid-template-rows: "auto 1fr"
	Gap     string `json:"gap,omitempty"`     // gap: "16px" | "8px 16px"
	Areas   string `json:"areas,omitempty"`   // grid-template-areas: '"header header" "sidebar main"'
	Justify string `json:"justify,omitempty"` // justify-items: "start" | "center" | "end" | "stretch"
	Align   string `json:"align,omitempty"`   // align-items: "start" | "center" | "end" | "stretch"

	// General properties
	Padding   string `json:"padding,omitempty"`   // padding: "16px"
	Width     string `json:"width,omitempty"`     // "full" | "reading" | "form" | "auto"
	ClassName string `json:"className,omitempty"` // Legacy class for published definitions

	// Responsive breakpoint overrides
	Responsive map[string]LayoutProps `json:"responsive,omitempty"`
}

// ============================================================================
// 2. Data Nodes
// ============================================================================

// DataSpec defines a data source scoped to a layout node.
// Child components reference it via "$id".
type DataSpec struct {
	EnabledWhen *Expression `json:"enabledWhen,omitempty"`
	PollWhen    *Expression `json:"pollWhen,omitempty"`
	NonBlocking bool        `json:"nonBlocking,omitempty"`
	ID          string      `json:"id"`                  // Variable name for $ references
	Provider    string      `json:"provider,omitempty"`  // Runtime Provider ID
	Input       []Mapping   `json:"input,omitempty"`     // Input mappings
	Transform   Expression  `json:"transform,omitempty"` // Optional data transformation
	Initial     any         `json:"initial,omitempty"`   // Loading/initial value
	PollMs      int         `json:"pollMs,omitempty"`    // Poll interval in milliseconds
	StaleMs     int         `json:"staleMs,omitempty"`   // Cache duration in milliseconds
}

// ============================================================================
// 3. Component Nodes
// ============================================================================

// ComponentSpec defines a pre-built UI component with props.
type ComponentSpec struct {
	Bindings []Mapping      `json:"bindings,omitempty"`
	Visible  *Expression    `json:"visible,omitempty"`
	Content  *LayoutSpec    `json:"content,omitempty"`
	ID       string         `json:"id"`               // Unique within the page
	Kind     string         `json:"kind"`             // Component type name (e.g. "data-table")
	Version  string         `json:"version"`          // Component version (e.g. "1.0")
	Props    map[string]any `json:"props"`            // Component-specific properties
	Data     string         `json:"data,omitempty"`   // Data reference: "$accounts"
	Events   []EventBinding `json:"events,omitempty"` // Event handlers
}

// EventBinding connects a component event to an action.
type EventBinding struct {
	When           *Expression       `json:"when,omitempty"`
	IdempotencyKey *Expression       `json:"idempotencyKey,omitempty"`
	ResultTarget   string            `json:"resultTarget,omitempty"`
	Result         *Expression       `json:"result,omitempty"`
	Disclosure     *Disclosure       `json:"disclosure,omitempty"`
	Reauthenticate *Reauthentication `json:"reauthenticate,omitempty"`
	Download       *DownloadResult   `json:"download,omitempty"`
	Event          string            `json:"event"`             // "click" | "change" | "submit"
	Action         string            `json:"action"`            // "navigate" | "invoke" | "setData" | "refresh"
	Target         string            `json:"target"`            // Page ID | Capability name | Data ID
	Payload        Expression        `json:"payload,omitempty"` // Event payload expression
	InputSchema    string            `json:"inputSchema,omitempty"`
}

// DownloadResult sends an approved read result to a local file.
type DownloadResult struct {
	Filename      string `json:"filename"`
	ContentField  string `json:"contentField,omitempty"`
	FilenameField string `json:"filenameField,omitempty"`
}

// Reauthentication passes a transient Gateway proof directly to its consuming action.
type Reauthentication struct {
	Capability CapabilityRef `json:"capability"`
	When       *Expression   `json:"when,omitempty"`
	Operation  string        `json:"operation"`
}

// Disclosure presents one transient string result without binding it to page data.
type Disclosure struct {
	Field          string `json:"field"`
	TitleKey       string `json:"titleKey"`
	CopyLabelKey   string `json:"copyLabelKey"`
	DismissKey     string `json:"dismissKey"`
	CopySuccessKey string `json:"copySuccessKey"`
	CopyErrorKey   string `json:"copyErrorKey"`
}

// ============================================================================
// Page Definition (new protocol)
// ============================================================================

// PageV3 is the new page definition using the v3 protocol.
type PageV3 struct {
	SchemaVersion string `json:"schemaVersion"` // Always "3.0"
	ID            string `json:"id"`
	Route         string `json:"route"`
	TitleKey      string `json:"titleKey"`
	SubtitleKey   string `json:"subtitleKey,omitempty"`
	Permission    string `json:"permission"`
	Root          Node   `json:"root"` // Must be Type: "layout"
}

// ApplicationV3 is the new application definition.
type ApplicationV3 struct {
	SchemaVersion        string       `json:"schemaVersion"` // Always "3.0"
	ID                   string       `json:"id"`
	Version              string       `json:"version"`
	Source               string       `json:"source"`
	RuntimeCompatibility string       `json:"runtimeCompatibility"`
	LabelKey             string       `json:"labelKey"`
	Permission           string       `json:"permission"`
	Route                string       `json:"route"`
	Pages                []PageV3     `json:"pages"`
	Providers            []Provider   `json:"providers"`
	Navigation           []Navigation `json:"navigation,omitempty"`
}

type ProtocolRoute struct {
	ApplicationID string `json:"applicationId"`
	PageID        string `json:"pageId"`
	Route         string `json:"route"`
	Permission    string `json:"permission"`
}

func ProtocolRoutes(applications []ApplicationV2) []ProtocolRoute {
	out := []ProtocolRoute{}
	for _, application := range applications {
		if application.Protocol == nil {
			continue
		}
		for _, page := range application.Protocol.Pages {
			out = append(out, ProtocolRoute{
				ApplicationID: application.ID, PageID: page.ID,
				Route: page.Route, Permission: page.Permission,
			})
		}
	}
	return out
}

func ProtocolCapabilityRefs(app ApplicationV3) []CapabilityRef {
	result := []CapabilityRef{}
	seen := map[string]bool{}
	add := func(ref CapabilityRef) {
		if ref.Name == "" || ref.Version == "" {
			return
		}
		key := ref.Name + "@" + ref.Version
		if seen[key] {
			return
		}
		seen[key] = true
		result = append(result, ref)
	}
	var visit func(any)
	visit = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			if capability, ok := node["capability"].(map[string]any); ok {
				add(CapabilityRef{Name: textValue(capability["name"]), Version: textValue(capability["version"])})
			}
			if action, ok := node["action"].(string); ok && action == "invoke" {
				if name, version, found := strings.Cut(textValue(node["target"]), "@"); found {
					add(CapabilityRef{Name: name, Version: version})
				}
			}
			for _, child := range node {
				visit(child)
			}
		case []any:
			for _, child := range node {
				visit(child)
			}
		}
	}
	raw, err := json.Marshal(app)
	if err != nil {
		return result
	}
	var document any
	if json.Unmarshal(raw, &document) != nil {
		return result
	}
	visit(document)
	return result
}

func ProtocolPageCapabilityRefs(page PageV3) []CapabilityRef {
	raw, err := json.Marshal(page)
	if err != nil {
		return []CapabilityRef{}
	}
	var document any
	if json.Unmarshal(raw, &document) != nil {
		return []CapabilityRef{}
	}
	result := []CapabilityRef{}
	seen := map[string]bool{}
	var visit func(any)
	visit = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			if capability, ok := node["capability"].(map[string]any); ok {
				ref := CapabilityRef{Name: textValue(capability["name"]), Version: textValue(capability["version"])}
				key := ref.Name + "@" + ref.Version
				if ref.Name != "" && ref.Version != "" && !seen[key] {
					seen[key] = true
					result = append(result, ref)
				}
			}
			if name, ok := node["capability"].(string); ok {
				version, _ := node["version"].(string)
				key := name + "@" + version
				if name != "" && version != "" && !seen[key] {
					seen[key] = true
					result = append(result, CapabilityRef{Name: name, Version: version})
				}
			}
			if action, ok := node["action"].(string); ok && action == "invoke" {
				if name, version, found := strings.Cut(textValue(node["target"]), "@"); found {
					key := name + "@" + version
					if !seen[key] {
						seen[key] = true
						result = append(result, CapabilityRef{Name: name, Version: version})
					}
				}
			}
			for _, child := range node {
				visit(child)
			}
		case []any:
			for _, child := range node {
				visit(child)
			}
		}
	}
	visit(document)
	return result
}

func textValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func protocolFieldComponents() map[string]bool {
	result := map[string]bool{}
	for _, name := range CoreFieldComponents() {
		result[name] = true
	}
	result["array"] = true
	result["membership-editor"] = true
	return result
}

func bindProtocolForm(value any, providers map[string]*Provider, definitions map[string]contracts.Definition) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var form FormV2
	if err = json.Unmarshal(raw, &form); err != nil {
		return err
	}
	if form.ID == "" || form.LabelKey == "" {
		return fmt.Errorf("form identity")
	}
	for commandName, command := range map[string]*Command{"create": &form.Create, "edit": &form.Edit} {
		if command.Capability.Name == "" {
			continue
		}
		definition, ok := definitions[command.Capability.Name+"@"+command.Capability.Version]
		if !ok || definition.ReadOnly {
			return fmt.Errorf("form %s %s capability", form.ID, commandName)
		}
		if command.ActionType == "" {
			command.ActionType = "capability"
		}
		if command.ActionType != "capability" && command.ActionType != "batch" {
			return fmt.Errorf("form %s action type", form.ID)
		}
		command.Permission = definition.Permission
		command.InputSchema = JSONSchema(definition.Input)
		if err := checkMappings(command.Input, definition.Input); err != nil {
			return fmt.Errorf("form %s %s input: %w", form.ID, commandName, err)
		}
		for _, field := range command.Fields {
			if field.Name == "" || field.LabelKey == "" || !protocolFieldComponents()[field.Component] || field.ComponentVersion != "2.0" {
				return fmt.Errorf("form %s field %s", form.ID, field.Name)
			}
			if field.Reference != "" && providers[field.Reference] == nil {
				return fmt.Errorf("form %s reference %s", form.ID, field.Reference)
			}
		}
	}
	if form.Create.Capability.Name == "" && form.Edit.Capability.Name == "" {
		return fmt.Errorf("form %s command", form.ID)
	}
	for reference := range form.ReferencePaths {
		if providers[reference] == nil {
			return fmt.Errorf("form %s reference path %s", form.ID, reference)
		}
	}
	raw, err = json.Marshal(form)
	if err != nil {
		return err
	}
	var replacement map[string]any
	if err = json.Unmarshal(raw, &replacement); err != nil {
		return err
	}
	return copyProtocolMap(value, replacement)
}

func copyProtocolMap(target any, replacement map[string]any) error {
	switch node := target.(type) {
	case map[string]any:
		for key, value := range replacement {
			node[key] = value
		}
		return nil
	default:
		return fmt.Errorf("form target")
	}
}

func bindProtocolForms(layout *LayoutSpec, providers map[string]*Provider, definitions map[string]contracts.Definition) error {
	for _, component := range layout.Children {
		if component.Layout != nil {
			if err := bindProtocolForms(component.Layout, providers, definitions); err != nil {
				return err
			}
			continue
		}
		if component.Component == nil {
			continue
		}
		if content := component.Component.Content; content != nil {
			if err := bindProtocolForms(content, providers, definitions); err != nil {
				return err
			}
		}
		props := component.Component.Props
		if value, ok := props["form"]; ok {
			if err := bindProtocolForm(value, providers, definitions); err != nil {
				return fmt.Errorf("component %s: %w", component.Component.ID, err)
			}
		}
		if value, ok := props["forms"].(map[string]any); ok {
			for key, form := range value {
				if err := bindProtocolForm(form, providers, definitions); err != nil {
					return fmt.Errorf("component %s form %s: %w", component.Component.ID, key, err)
				}
			}
		}
	}
	return nil
}

// ============================================================================
// Validation
// ============================================================================

var (
	validLayoutKinds = map[string]bool{
		"grid": true, "stack": true, "sidebar": true, "tabs": true, "split": true,
	}
	validEventActions = map[string]bool{
		"navigate": true, "invoke": true, "setData": true, "refresh": true, "system": true, "browser": true,
	}
)

// ValidateProtocol validates an ApplicationV3 against the protocol rules.
func ValidateProtocol(app ApplicationV3) error {
	if app.SchemaVersion != "3.0" {
		return fmt.Errorf("schema_version_unsupported: expected 3.0, got %s", app.SchemaVersion)
	}
	if app.ID == "" || app.Source == "" || app.Permission == "" {
		return fmt.Errorf("schema_invalid: id, source, permission required")
	}
	pageIDs := map[string]bool{}
	for _, page := range app.Pages {
		if pageIDs[page.ID] {
			return fmt.Errorf("schema_invalid: duplicate page id %s", page.ID)
		}
		pageIDs[page.ID] = true
		if page.SchemaVersion != "3.0" {
			return fmt.Errorf("schema_version_unsupported: page %s", page.ID)
		}
		if page.Root.Type != "layout" || page.Root.Layout == nil {
			return fmt.Errorf("schema_invalid: page %s root must be layout", page.ID)
		}
		providerIDs := map[string]bool{}
		for _, p := range app.Providers {
			providerIDs[p.ID] = true
		}
		if err := validateLayout(page.Root.Layout, providerIDs, nil, 0, 10); err != nil {
			return fmt.Errorf("page %s: %w", page.ID, err)
		}
	}
	return nil
}

var batchImportTemplatePath = regexp.MustCompile(batchImportTemplatePathPattern)

var layoutClassPattern = regexp.MustCompile(`^[A-Za-z0-9 _-]{0,160}$`)

func validateStyleString(name, value string) error {
	if len(value) > 500 {
		return fmt.Errorf("style budget %s", name)
	}
	lower := strings.ToLower(value)
	for _, unsafe := range []string{"url(", "javascript:", "expression(", "<", ">", "&"} {
		if strings.Contains(lower, unsafe) {
			return fmt.Errorf("style unsafe %s", name)
		}
	}
	return nil
}

func validateLayoutProps(props LayoutProps) error {
	for name, value := range map[string]string{
		"columns": props.Columns, "rows": props.Rows, "gap": props.Gap,
		"areas": props.Areas, "padding": props.Padding, "className": props.ClassName,
	} {
		if err := validateStyleString(name, value); err != nil {
			return err
		}
	}
	if !layoutClassPattern.MatchString(props.ClassName) {
		return fmt.Errorf("style class")
	}
	for breakpoint, override := range props.Responsive {
		if err := validateLayoutProps(override); err != nil {
			return fmt.Errorf("responsive %s: %w", breakpoint, err)
		}
	}
	return nil
}

// systemProviderInput defines the fixed system projections available to V3.
func systemProviderInput(adapter string) (*contracts.Schema, string) {
	switch adapter {
	case "system.capability-definitions", "system.capability-approvals":
		return contracts.Object(nil), ""
	case "system.capability-invocations", "system.capability-events":
		return contracts.Object(map[string]*contracts.Schema{"limit": contracts.Int(1, 100)}), ""
	case "system.ai-status":
		return contracts.Object(nil), "assistant:use"
	default:
		return nil, ""
	}
}

// PublicSystemProvider reports whether this fixed system projection needs only an authenticated account.
func PublicSystemProvider(adapter string) bool {
	schema, permission := systemProviderInput(adapter)
	return schema != nil && permission == ""
}

// assembleProtocol binds V3 declarations to the same immutable capability
// catalog used by V2. Presentation schema never grants a permission.
func assembleProtocol(wrapper *ApplicationV2, definitions map[string]contracts.Definition) error {
	app := wrapper.Protocol
	if wrapper.ID != app.ID || wrapper.Version != app.Version || wrapper.Source != app.Source ||
		wrapper.Route != app.Route || wrapper.Permission != app.Permission || wrapper.LabelKey != app.LabelKey {
		return fmt.Errorf("wrapper metadata mismatch")
	}
	providers := map[string]*Provider{}
	for i := range app.Providers {
		provider := &app.Providers[i]
		if schema, permission := systemProviderInput(provider.Adapter); schema != nil {
			if provider.ID == "" || providers[provider.ID] != nil || provider.Capability != (CapabilityRef{}) {
				return fmt.Errorf("system provider %s", provider.ID)
			}
			if err := checkMappings(provider.Input, schema); err != nil {
				return fmt.Errorf("provider %s: %w", provider.ID, err)
			}
			provider.Permission = permission
			provider.InputSchema = JSONSchema(schema)
			provider.OutputSchema = ""
			providers[provider.ID] = provider
			continue
		}
		if provider.Adapter != "" && provider.Adapter != "capability" {
			return fmt.Errorf("provider %s unknown adapter", provider.ID)
		}
		definition, ok := definitions[provider.Capability.Name+"@"+provider.Capability.Version]
		if !ok || !definition.ReadOnly || providers[provider.ID] != nil || provider.KeyField == "" {
			return fmt.Errorf("provider %s", provider.ID)
		}
		if provider.Adapter == "" {
			provider.Adapter = "capability"
		}
		provider.Permission = definition.Permission
		if provider.Permission == "" && app.Permission == AuthenticatedPermission &&
			strings.HasPrefix(app.Source, "builtin:") && slices.Contains(definition.Tags, contracts.AuthenticatedProviderTag) {
			provider.Permission = AuthenticatedPermission
		}
		provider.InputSchema = JSONSchema(definition.Input)
		projected, err := providerSchema(definition.Output, *provider)
		if err != nil {
			return fmt.Errorf("provider %s: %w", provider.ID, err)
		}
		provider.OutputSchema = JSONSchema(projected)
		if err := checkMappings(provider.Input, definition.Input); err != nil {
			return fmt.Errorf("provider %s: %w", provider.ID, err)
		}
		providers[provider.ID] = provider
	}
	if err := ValidateProtocol(*app); err != nil {
		return err
	}
	pageIDs := map[string]bool{}
	for _, page := range app.Pages {
		pageIDs[page.ID] = true
	}
	for _, page := range app.Pages {
		if err := bindProtocolForms(page.Root.Layout, providers, definitions); err != nil {
			return fmt.Errorf("page %s: %w", page.ID, err)
		}
		allowBrowser := app.ID == "agent-collaboration" && app.Source == "builtin:agent-collaboration" && page.Route == "/mcp"
		if err := validateProtocolLayout(page.Root, providers, definitions, pageIDs, nil, 0, allowBrowser); err != nil {
			return fmt.Errorf("page %s: %w", page.ID, err)
		}
		if err := validateAIPage(page.Root, providers, definitions); err != nil {
			return fmt.Errorf("page %s: %w", page.ID, err)
		}
	}
	wrapper.Pages = []PageV2{}
	wrapper.Providers = []Provider{}
	wrapper.Extensions = []ExtensionV2{}
	wrapper.Navigation = app.Navigation
	wrapper.MetadataVersion = contracts.Digest(contracts.Input{"definition": Canonical(*wrapper)})
	return nil
}

func validateProtocolLayout(
	node Node,
	providers map[string]*Provider,
	definitions map[string]contracts.Definition,
	pageIDs map[string]bool,
	dataScope map[string]bool,
	depth int,
	allowBrowser bool,
) error {
	if depth > 10 || node.Type != "layout" || node.Layout == nil {
		return fmt.Errorf("layout")
	}
	layout := node.Layout
	scope := map[string]bool{}
	for key, value := range dataScope {
		scope[key] = value
	}
	for _, data := range layout.Data {
		mappings := data.Input
		provider := providers[data.Provider]
		if provider == nil && data.Provider != "" {
			return fmt.Errorf("data %s provider", data.ID)
		}
		if provider != nil {
			if len(mappings) == 0 {
				mappings = provider.Input
			}
			schema, _ := systemProviderInput(provider.Adapter)
			if schema == nil {
				schema = definitions[provider.Capability.Name+"@"+provider.Capability.Version].Input
			}
			if err := checkMappings(mappings, schema); err != nil {
				return fmt.Errorf("data %s input: %w", data.ID, err)
			}
		}
		for _, expression := range []*Expression{&data.Transform, data.EnabledWhen, data.PollWhen} {
			if expression == nil {
				continue
			}
			if err := ValidateExpression(*expression, 0); err != nil {
				return fmt.Errorf("data %s expression: %w", data.ID, err)
			}
		}
		scope[data.ID] = true
	}
	componentCount := 0
	for _, child := range layout.Children {
		if child.Type == "layout" {
			if err := validateProtocolLayout(child, providers, definitions, pageIDs, scope, depth+1, allowBrowser); err != nil {
				return err
			}
			continue
		}
		component := child.Component
		if child.Type != "component" || component == nil {
			return fmt.Errorf("node type")
		}
		componentCount++
		if componentCount > 1 {
			return fmt.Errorf("layout %s component count", layout.ID)
		}
		if _, ok := trustedUIContracts[component.Kind+"@"+component.Version]; !ok {
			return fmt.Errorf("component %s kind", component.ID)
		}
		if component.Content != nil {
			if err := validateProtocolLayout(Node{Type: "layout", Layout: component.Content}, providers, definitions, pageIDs, scope, depth+1, allowBrowser); err != nil {
				return err
			}
		}
		if defaults, exists := component.Props["browserDefaults"]; exists {
			if !allowBrowser || component.Kind != "action-form" || defaults != "webmcp-pairing" ||
				!slices.ContainsFunc(component.Events, func(event EventBinding) bool {
					return event.Event == "submit" && event.Action == "browser" && event.Target == "webmcp.connect"
				}) {
				return fmt.Errorf("component %s has invalid browser defaults", component.ID)
			}
		}
		for eventIndex := range component.Events {
			event := &component.Events[eventIndex]
			for _, expression := range []*Expression{&event.Payload, event.When, event.IdempotencyKey, event.Result} {
				if expression == nil {
					continue
				}
				if err := ValidateExpression(*expression, 0); err != nil {
					return fmt.Errorf("component %s event: %w", component.ID, err)
				}
			}
			if event.ResultTarget != "" && (!strings.HasPrefix(event.ResultTarget, "_") || !scope[event.ResultTarget] || event.Result == nil || event.Result.Kind == "") {
				return fmt.Errorf("component %s result target", component.ID)
			}
			if (event.Disclosure != nil || event.Reauthenticate != nil || event.Download != nil) && event.Action != "invoke" {
				return fmt.Errorf("component %s disclosure requires an invocation", component.ID)
			}
			switch event.Action {
			case "navigate":
				if !pageIDs[event.Target] {
					return fmt.Errorf("component %s navigation target", component.ID)
				}
			case "setData", "refresh":
				if !scope[event.Target] {
					return fmt.Errorf("component %s data target", component.ID)
				}
			case "browser":
				if err := validateBrowserAction(component, event, allowBrowser); err != nil {
					return fmt.Errorf("component %s: %w", component.ID, err)
				}
			case "system":
				if event.Target != "capability.approval.decide" {
					return fmt.Errorf("component %s system action", component.ID)
				}
				event.InputSchema = JSONSchema(contracts.Object(map[string]*contracts.Schema{
					"id": contracts.Int(1, 9007199254740991), "approve": {Type: "boolean"},
				}, "id", "approve"))
			case "invoke":
				name, version, found := strings.Cut(event.Target, "@")
				definition, definitionFound := definitions[name+"@"+version]
				if !found || !definitionFound {
					return fmt.Errorf("component %s invoke target", component.ID)
				}
				if err := validateInvocationResult(*event, definition); err != nil {
					return fmt.Errorf("component %s: %w", component.ID, err)
				}
				if err := validateInvocationReauthentication(component, event, definitions); err != nil {
					return fmt.Errorf("component %s: %w", component.ID, err)
				}
				event.InputSchema = JSONSchema(definition.Input)
			}
		}
	}
	return nil
}

func validateInvocationResult(event EventBinding, definition contracts.Definition) error {
	if definition.TransientResult && (event.ResultTarget != "" || event.Result != nil) {
		return fmt.Errorf("transient invocation results cannot bind to page data")
	}
	if event.Download != nil && (!definition.ReadOnly || definition.TransientResult ||
		event.Result != nil || event.ResultTarget != "" || !validDownload(*event.Download, definition.Output)) {
		return fmt.Errorf("download requires a read-only, non-transient invocation and a JSON filename")
	}
	d := event.Disclosure
	if d == nil {
		return nil
	}
	if !definition.TransientResult || definition.ReadOnly || definition.Output == nil {
		return fmt.Errorf("disclosure requires a transient write result")
	}
	field := definition.Output.Properties[d.Field]
	if field == nil || field.Type != "string" {
		return fmt.Errorf("disclosure requires a declared string result field")
	}
	if d.TitleKey == "" || d.CopyLabelKey == "" || d.DismissKey == "" || d.CopySuccessKey == "" || d.CopyErrorKey == "" {
		return fmt.Errorf("disclosure requires localized controls and feedback")
	}
	return nil
}

// validateLayout recursively validates a layout node and its children.
func validateLayout(layout *LayoutSpec, providers, dataScope map[string]bool, depth, maxDepth int) error {
	if depth > maxDepth {
		return fmt.Errorf("schema_invalid: max nesting depth %d exceeded", maxDepth)
	}
	if layout.ID == "" {
		return fmt.Errorf("schema_invalid: layout id required")
	}
	if !validLayoutKinds[layout.Kind] {
		return fmt.Errorf("schema_invalid: unknown layout kind %q", layout.Kind)
	}
	if layout.Visible != nil {
		if err := ValidateExpression(*layout.Visible, 0); err != nil {
			return err
		}
	}
	if err := validateLayoutProps(layout.Props); err != nil {
		return fmt.Errorf("schema_invalid: layout %s %w", layout.ID, err)
	}

	// Merge parent data scope with this layout's data
	scope := map[string]bool{}
	for k, v := range dataScope {
		scope[k] = v
	}
	for _, data := range layout.Data {
		if data.ID == "" {
			return fmt.Errorf("schema_invalid: data id required in layout %s", layout.ID)
		}
		if data.ID[0] != '_' && data.Provider == "" {
			return fmt.Errorf("schema_invalid: data %s missing provider", data.ID)
		}
		if data.Provider != "" && !providers[data.Provider] {
			return fmt.Errorf("schema_invalid: data %s references unknown provider %s", data.ID, data.Provider)
		}
		if scope[data.ID] {
			return fmt.Errorf("schema_invalid: duplicate data id %s", data.ID)
		}
		scope[data.ID] = true
	}

	// Validate nesting rule: at most 1 component child, 0-N layout children
	componentCount := 0
	for i, child := range layout.Children {
		switch child.Type {
		case "component":
			if child.Component == nil {
				return fmt.Errorf("schema_invalid: child %d missing component spec", i)
			}
			componentCount++
			if componentCount > 1 {
				return fmt.Errorf("schema_invalid: layout %s has %d components, max 1", layout.ID, componentCount)
			}
			if err := validateComponent(child.Component, scope); err != nil {
				return fmt.Errorf("layout %s child %d: %w", layout.ID, i, err)
			}
			if content := child.Component.Content; content != nil {
				if err := validateLayout(content, providers, scope, depth+1, maxDepth); err != nil {
					return err
				}
			}
		case "layout":
			if child.Layout == nil {
				return fmt.Errorf("schema_invalid: child %d missing layout spec", i)
			}
			if err := validateLayout(child.Layout, providers, scope, depth+1, maxDepth); err != nil {
				return fmt.Errorf("layout %s child %d: %w", layout.ID, i, err)
			}
		default:
			return fmt.Errorf("schema_invalid: unknown node type %q", child.Type)
		}
	}
	return nil
}

// validateComponent validates a component node.
func validateComponent(comp *ComponentSpec, dataScope map[string]bool) error {
	if comp.ID == "" {
		return fmt.Errorf("schema_invalid: component id required")
	}
	if comp.Kind == "" {
		return fmt.Errorf("schema_invalid: component %s missing kind", comp.ID)
	}
	if comp.Version == "" {
		return fmt.Errorf("schema_invalid: component %s missing version", comp.ID)
	}
	contract, registered := trustedUIContracts[comp.Kind+"@"+comp.Version]
	if !registered {
		return fmt.Errorf("component_not_registered: %s@%s", comp.Kind, comp.Version)
	}
	if comp.Kind == "batch-import" {
		templatePath, _ := comp.Props["templatePath"].(string)
		if !batchImportTemplatePath.MatchString(templatePath) {
			return fmt.Errorf("schema_invalid: component %s requires a valid templatePath", comp.ID)
		}
		for _, binding := range comp.Bindings {
			if binding.Target == "templatePath" {
				return fmt.Errorf("schema_invalid: component %s templatePath must be static", comp.ID)
			}
		}
	}
	if class, ok := comp.Props["className"].(string); ok &&
		(!layoutClassPattern.MatchString(class) || len(class) > 160) {
		return fmt.Errorf("schema_invalid: component class")
	}
	if comp.Visible != nil {
		if err := ValidateExpression(*comp.Visible, 0); err != nil {
			return err
		}
	}
	if len(comp.Bindings) > 0 {
		if err := ValidateExpression(Expression{Kind: "object", Fields: comp.Bindings}, 0); err != nil {
			return err
		}
	}
	if comp.Content != nil && !slices.Contains(contract.Slots, "content") {
		return fmt.Errorf("schema_invalid: component %s does not accept content", comp.ID)
	}
	if _, ok := comp.Props["style"]; ok {
		return fmt.Errorf("schema_invalid: component style")
	}
	if err := validateShadcnComponent(comp); err != nil {
		return err
	}
	if comp.Kind == "action-form" {
		submits := 0
		for _, event := range comp.Events {
			if event.Event != "submit" {
				continue
			}
			submits++
			if event.Action != "invoke" && event.Action != "browser" {
				return fmt.Errorf("schema_invalid: action form submit must invoke a declared operation")
			}
		}
		if submits > 1 {
			return fmt.Errorf("schema_invalid: action form must have at most one submit binding")
		}
	}
	// Validate data reference
	if comp.Data != "" {
		ref := strings.TrimPrefix(comp.Data, "$")
		if !strings.HasPrefix(comp.Data, "$") {
			return fmt.Errorf("schema_invalid: component %s data ref must start with $", comp.ID)
		}
		if !dataScope[ref] {
			return fmt.Errorf("schema_invalid: component %s references undefined data %s", comp.ID, comp.Data)
		}
	}
	// Validate events
	for _, evt := range comp.Events {
		if evt.Event == "" || evt.Action == "" || evt.Target == "" {
			return fmt.Errorf("schema_invalid: component %s event missing fields", comp.ID)
		}
		if !validEventActions[evt.Action] {
			return fmt.Errorf("schema_invalid: component %s unknown event action %q", comp.ID, evt.Action)
		}
	}
	return nil
}

func providerSchema(source *contracts.Schema, provider Provider) (*contracts.Schema, error) {
	result := Canonical(*source)
	if len(provider.Select) == 0 {
		return &result, nil
	}
	row, err := schemaPath(&result, provider.ResultPath)
	if err != nil {
		return nil, err
	}
	if row.Type == "array" {
		row = row.Items
	}
	if row == nil || row.Type != "object" {
		return nil, fmt.Errorf("schema_invalid")
	}
	keep := map[string]bool{provider.KeyField: true}
	for _, name := range provider.Select {
		keep[name] = true
	}
	for name := range row.Properties {
		if !keep[name] {
			delete(row.Properties, name)
		}
	}
	required := []string{}
	for _, name := range row.Required {
		if keep[name] {
			required = append(required, name)
		}
	}
	row.Required = required
	return &result, nil
}
