package main

var definitionDescriptions = map[string]struct {
	summary string
	fields  map[string]string
}{
	"CapabilitySchema": {"Gateway's supported schema vocabulary for business values.", map[string]string{
		"type": "Expected JSON value type.", "description": "Business meaning supplied by the defining module.",
		"properties": "Named child field schemas.", "required": "Fields that must be present.",
		"items": "Array element schema.", "enum": "Allowed string values.",
		"additionalProperties": "Whether undeclared object fields are permitted.",
		"minLength":            "Minimum string length.", "maxLength": "Maximum string length.",
		"minimum": "Minimum numeric value.", "maximum": "Maximum numeric value.",
		"minItems": "Minimum array length.", "maxItems": "Maximum array length.",
	}},
	"FieldAccess": {"Field-level permission rule.", map[string]string{
		"Path": "Schema path of the protected field.", "Permission": "Permission required for access.",
	}},
	"Idempotency": {"Replay contract for an invocation.", map[string]string{
		"Required": "Whether callers must supply an idempotency key.", "Scope": "Declared scope for matching replay keys.",
	}},
	"GraphRead": {"One capability available to a bounded Graph.", map[string]string{
		"capability": "Name of the registered read capability.", "version": "Pinned capability contract version.",
		"input": "Maps capability argument names to Graph input fields.",
	}},
	"GraphReference": {"Pinned Graph identity and content digest.", map[string]string{
		"ref": "Graph name and version.", "hash": "Expected definition digest.",
	}},
	"Mapping": {"Workflow value mapping.", map[string]string{
		"from": "Workflow input or source node identifier.", "field": "Field path in the source value.",
	}},
	"Poll": {"Bounded polling of a capability result.", map[string]string{
		"doneField": "Result field indicating completion.", "maxSteps": "Maximum poll iterations.",
	}},
	"Agent": {"Agent node instructions, available tools and budget.", map[string]string{
		"instruction": "Prompt instruction for the Agent node.", "tools": "Allowed capability keys pinned by version.",
		"maxIterations": "Maximum Agent iterations.", "output": "Optional structured output contract.",
	}},
	"Compensation": {"Capability invoked to compensate a prior step.", map[string]string{
		"capability": "Registered compensation capability.", "version": "Pinned capability version.",
		"input": "Mapped compensation inputs.", "constants": "Literal compensation arguments.",
	}},
	"When": {"A condition evaluated against an earlier node result.", map[string]string{
		"from": "Source node identifier.", "field": "Field path in the source result.", "equals": "Expected comparison value.",
	}},
	"Step": {"A workflow node invoking a capability, Graph or Agent.", nil},
	"RuntimeProtocol": {"Application Runtime V3 protocol.", map[string]string{
		"schemaVersion": "Version of the V3 wire protocol.", "id": "Stable application identifier.",
		"version": "Application contribution version.", "source": "Owning application or plugin source.",
		"runtimeCompatibility": "Supported Runtime version requirement.", "labelKey": "Localized application label key.",
		"permission": "Permission required to open the application.", "route": "Base application route.",
		"pages": "V3 page definitions.", "providers": "Application data providers.", "navigation": "Application navigation items.",
	}},
	"RuntimePage": {"A V3 page with a route, permission and root node.", map[string]string{
		"schemaVersion": "Version of the V3 wire protocol.", "id": "Stable page identifier within the application.",
		"route": "Route pattern, including any route parameters.", "titleKey": "Localized page title key.",
		"subtitleKey": "Localized page subtitle key.", "permission": "Permission required to open the page.",
		"root": "Root of the layout and component tree.",
	}},
	"RuntimeNode": {"Discriminated Runtime layout or component node.", map[string]string{
		"type": "Node discriminator: layout or component.", "layout": "Layout variant.", "component": "Component variant.",
	}},
	"RuntimeLayout": {"Container for child nodes and scoped data bindings.", map[string]string{
		"id": "Stable layout identifier.", "kind": "Layout variant.", "props": "Layout sizing and presentation.",
		"data": "Data sources scoped to this layout.", "children": "Child layout or component nodes.",
		"visible": "Condition governing visibility.",
	}},
	"RuntimeLayoutProps": {"Layout sizing, spacing and responsive overrides.", map[string]string{
		"columns": "Grid column template.", "rows": "Grid row template.", "areas": "Named grid areas.",
		"align": "Cross-axis alignment.", "justify": "Main-axis alignment.", "gap": "Spacing between children.",
		"padding": "Padding inside the container.", "width": "Layout width preset.",
		"className": "Legacy class for published definitions.", "responsive": "Layout overrides keyed by breakpoint.",
	}},
	"RuntimeComponent": {"Trusted component instance and its bindings.", map[string]string{
		"id": "Unique component identifier within the page.", "kind": "Trusted component kind.",
		"version": "Component contract version.", "props": "Properties interpreted by the registered component.",
		"data": "Reference to scoped layout data.", "bindings": "Mappings into component props.",
		"visible": "Condition governing visibility.", "events": "Event-to-action bindings.",
		"content": "Optional nested layout for container components.",
	}},
	"RuntimeEvent": {"An event action and its input, condition and result bindings.", map[string]string{
		"event": "Component event name.", "action": "Runtime action kind.", "target": "Target for the selected action.",
		"payload": "Expression producing action arguments.", "inputSchema": "Serialized input contract for the action.",
		"when": "Condition for running the action.", "idempotencyKey": "Expression producing the idempotency key.",
		"resultTarget": "Destination for the action result.", "result": "Expression projecting the action result.",
	}},
	"RuntimeData": {"A provider invocation or local data slot scoped to a layout.", map[string]string{
		"id": "Scoped data name referenced by children.", "provider": "Application provider ID.",
		"input": "Provider argument mappings.", "transform": "Projection applied to the result.",
		"initial": "Initial data value.", "enabledWhen": "Condition enabling data reads.",
		"pollWhen": "Condition enabling polling.", "pollMs": "Poll interval in milliseconds.",
		"staleMs": "Staleness window in milliseconds.", "nonBlocking": "Whether other page content can render while loading.",
	}},
	"RuntimeProvider": {"Application-scoped data provider and its contract.", map[string]string{
		"id": "Provider identifier referenced by page data.", "adapter": "Provider adapter kind.",
		"capability": "Registered capability reference for the capability adapter.", "permission": "Required provider permission.",
		"inputSchema": "Serialized argument schema.", "outputSchema": "Serialized result schema.",
		"input": "Default input bindings.", "resultPath": "Path of provider data in the result.",
		"keyField": "Stable record key.", "labelField": "Record label field.", "pagination": "Pagination mode.",
		"pageSize": "Default page size.", "pollInterval": "Polling interval in milliseconds.",
		"filterTarget": "Filter destination in the provider input.", "queryBindings": "Bindings for query state.",
		"timeoutMs": "Timeout in milliseconds.", "staleTimeMs": "Cache staleness in milliseconds.",
		"select": "Selected fields.", "maxDepth": "Maximum configured nesting depth.", "tags": "Provider metadata tags.",
	}},
	"RuntimeExpression": {"Declarative expression for Runtime values and mappings.", map[string]string{
		"kind": "Expression discriminator.", "path": "Runtime data or event path.",
		"literal": "Literal value represented as JSON text.", "items": "Child expressions.",
		"fields": "Mappings that construct an object value.",
	}},
	"RuntimeField": {"Runtime field presentation and validation metadata, including nested fields.", map[string]string{
		"name": "Field name.", "labelKey": "Localized field label key.", "type": "Field value type.",
		"component": "Field renderer kind.", "componentVersion": "Field renderer version.",
		"required": "Whether a value must be provided.", "readOnly": "Whether users can edit the field.",
		"sensitive": "Whether the field is marked sensitive.", "derived": "Whether the value is derived.",
		"hidden": "Whether the field is hidden by default.", "visibleWhen": "Conditional visibility.",
		"disabledWhen": "Conditional disabling.", "compute": "Expression for a computed value.",
		"dependsOn": "Fields needed by the computation.", "children": "Nested fields.",
		"minLength": "Minimum text length.", "maxLength": "Maximum text length.",
		"minimum": "Minimum numeric value.", "maximum": "Maximum numeric value.",
		"enum": "Declared options.", "valueLabelPrefix": "Prefix used to localize option values.",
		"default": "Expression for the initial value.", "reference": "Referenced data source or relation.",
		"linkPage": "Page target for a field link.", "width": "Configured display width.",
		"display": "Presentation mode.", "fixed": "Whether the field is fixed in its container.",
		"semantic": "Whether the field participates in semantic context.",
		"minItems": "Minimum number of items.", "maxItems": "Maximum number of items.",
		"multiple": "Whether multiple values are accepted.", "searchParam": "Search parameter for reference values.",
		"group": "Display group.", "timezone": "Configured time zone.",
		"permission": "Permission required to display the field.", "writePermission": "Permission required to write it.",
	}},
}
