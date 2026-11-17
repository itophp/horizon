package main

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	js "github.com/google/jsonschema-go/jsonschema"
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/kernel/workflow/spec"
)

type schemaGuide struct {
	Kind        string     `json:"kind"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Source      string     `json:"source"`
	Rules       []string   `json:"rules"`
	Schema      *js.Schema `json:"schema"`
	Entry       *item      `json:"entry,omitempty"`
}

var schemaKinds = []string{"capability", "workflow", "graph", "application", "page"}

func describeFields(schema *js.Schema, fields map[string]string) {
	for name, description := range fields {
		if property := schema.Properties[name]; property != nil {
			property.Description = description
		}
	}
}

// ForType rejects cycles. Give the recursive Runtime and capability types
// stable local references and infer each definition's fields independently.
func inferSchema(root reflect.Type, recursive map[string]reflect.Type) (*js.Schema, error) {
	refs := map[reflect.Type]*js.Schema{}
	for name, t := range recursive {
		refs[t] = &js.Schema{Type: "object", Ref: "#/$defs/" + name}
	}
	options := &js.ForOptions{TypeSchemas: refs}
	schema, err := js.ForType(root, options)
	if err != nil {
		return nil, err
	}
	schema.Defs = map[string]*js.Schema{}
	for name, t := range recursive {
		def := &js.Schema{Type: "object", Properties: map[string]*js.Schema{}, AdditionalProperties: &js.Schema{Not: &js.Schema{}}}
		for _, field := range reflect.VisibleFields(t) {
			key, tag, _ := strings.Cut(field.Tag.Get("json"), ",")
			if key == "-" || !field.IsExported() {
				continue
			}
			if key == "" {
				key = field.Name
			}
			property, err := js.ForType(field.Type, options)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", name, key, err)
			}
			def.Properties[key] = property
			if tag != "omitempty" && tag != "omitzero" {
				def.Required = append(def.Required, key)
			}
		}
		schema.Defs[name] = def
	}
	return schema, nil
}

func schemaFor(kind string) (schemaGuide, error) {
	g := schemaGuide{Kind: kind, Source: "compiled"}
	var err error
	var fields map[string]string
	switch kind {
	case "capability":
		g.Title = "Capability definition"
		g.Description = "The versioned Gateway contract for a single action, including request and result schemas, access policy and declared side effects."
		g.Rules = []string{"Use Name and Version together when referencing an action.", "Input and Output describe business data. Authorization, data scope, risk and approval are enforced by Gateway at execution time.", "InputAccess and OutputAccess can restrict fields even when the contract declares them."}
		g.Schema, err = inferSchema(reflect.TypeFor[contracts.Definition](), map[string]reflect.Type{
			"CapabilitySchema": reflect.TypeFor[contracts.Schema](),
			"FieldAccess":      reflect.TypeFor[contracts.FieldAccess](),
			"Idempotency":      reflect.TypeFor[contracts.Idempotency](),
		})
		fields = map[string]string{
			"Name":            "Stable action identifier.",
			"Version":         "Version of this action contract.",
			"Title":           "Display title.",
			"Description":     "Business purpose of the registered action.",
			"Domain":          "Owning functional domain.",
			"Entity":          "Primary entity affected or queried.",
			"Entities":        "Additional declared entity scope, with the primary entity first.",
			"Input":           "Gateway request payload schema.",
			"Output":          "Business result data schema, when declared.",
			"Permission":      "Permission checked by Gateway.",
			"AllowedOrigins":  "Explicit invocation source allowlist, when configured.",
			"Risk":            "Risk classification used by policy.",
			"Approval":        "Approval policy.",
			"Idempotency":     "Idempotency requirement and key scope.",
			"ReadOnly":        "Whether the action is declared read-only.",
			"RetrySafe":       "Whether the declared execution can be retried safely.",
			"SuccessEvents":   "Event types produced on success.",
			"Loggable":        "Input fields allowed in audit projection.",
			"TransientResult": "Whether results are excluded from persistence and replay.",
			"Layer":           "Atomic action or orchestrating action.",
			"Kind":            "Executor category.",
			"Reads":           "Declared entity reads.",
			"Writes":          "Declared entity writes.",
			"Effect":          "Declared side-effect category.",
			"Executor":        "Implementation reference.",
			"Tags":            "Discovery and execution policy markers.",
			"Installation":    "Deployment-owned installation reference.",
			"Application":     "Deployment-owned application reference.",
			"InputAccess":     "Per-field input authorization rules.",
			"OutputAccess":    "Per-field output authorization rules.",
		}
	case "workflow":
		g.Title = "Workflow definition"
		g.Description = "A versioned executable workflow contract with an input schema, ordered dependencies, and nodes that reference registered capabilities, Graphs or Agent tools."
		g.Rules = []string{"Map inputs and dependencies explicitly. Capability and Graph references must pin their versions.", "Use validate workflow on a candidate file to check registry references and workflow semantics."}
		g.Schema, err = spec.SpecificationSchema()
		fields = map[string]string{
			"name":         "Stable registered workflow identifier.",
			"version":      "Version referenced by callers and execution plans.",
			"input":        "Workflow input payload schema.",
			"steps":        "Nodes and their declared dependencies and bindings.",
			"resultStep":   "Node selected as the workflow result, when declared.",
			"freshContext": "Request a fresh context for workflow execution.",
		}
		if err == nil {
			describeFields(g.Schema.Defs["Step"], map[string]string{
				"id":           "Node identifier used by dependencies and mappings.",
				"capability":   "Registered capability invoked by this node.",
				"version":      "Pinned version of the capability contract.",
				"graph":        "Pinned Graph reference and definition digest.",
				"agent":        "Agent instructions, tool allowlist and iteration budget.",
				"dependsOn":    "Nodes that must complete first.",
				"input":        "Mappings from workflow input or prior node results.",
				"constants":    "Literal argument values.",
				"when":         "Condition evaluated from a prior node result.",
				"compensation": "Declared compensation action for this node.",
				"poll":         "Bounded polling configuration.",
				"maxAttempts":  "Retry budget for eligible reads.",
				"callMode":     "How a capability execution plan is selected.",
				"planId":       "Pinned execution plan identifier, when applicable.",
				"planVersion":  "Pinned execution plan version, when applicable.",
			})
		}
	case "graph":
		g.Title = "Graph definition"
		g.Description = "A versioned bounded analysis Graph with input and output contracts, model instructions, and an allowlist of pinned read capabilities."
		g.Rules = []string{"A Graph's declared reads must use eligible read-only capabilities.", "Call, token, context and time budgets bound the execution.", "Workflow Graph references pin both the key and definition digest."}
		g.Schema, err = inferSchema(reflect.TypeFor[spec.GraphDefinition](), map[string]reflect.Type{
			"CapabilitySchema": reflect.TypeFor[contracts.Schema](),
			"GraphRead":        reflect.TypeFor[spec.GraphRead](),
		})
		fields = map[string]string{
			"name":            "Registered Graph identifier.",
			"version":         "Version used in pinned references.",
			"input":           "Graph request schema.",
			"output":          "Structured analysis result schema.",
			"reads":           "Allowed capability reads and their input bindings.",
			"instruction":     "Model instruction for the analysis.",
			"maxModelCalls":   "Maximum number of model calls.",
			"maxToolCalls":    "Maximum number of tool calls.",
			"maxOutputTokens": "Maximum output token budget.",
			"maxContextBytes": "Maximum context size in bytes.",
			"timeoutMillis":   "Execution time budget in milliseconds.",
		}
	case "application", "page":
		root := reflect.TypeFor[ar.ApplicationV2]()
		g.Title = "Runtime application definition"
		g.Description = "The compiled Runtime application envelope containing the V3 protocol, version and source metadata, providers, navigation and pages."
		g.Rules = []string{"Use protocol.pages and protocol.providers for V3 Runtime definitions.", "Version and source describe a compiled contribution. Tenant activation and permissions are resolved by the running platform."}
		fields = map[string]string{
			"protocol":             "V3 application, including pages, providers and navigation.",
			"schemaVersion":        "Runtime protocol schema version.",
			"id":                   "Stable Runtime application identifier.",
			"version":              "Compiled contribution version.",
			"source":               "Deployment or plugin contribution source.",
			"metadataVersion":      "Version of application metadata.",
			"permission":           "Application permission requirement.",
			"route":                "Application route prefix.",
			"labelKey":             "Localization key for the application label.",
			"agentTools":           "Declared capability references offered as application tools.",
			"messages":             "Application-owned localization strings.",
			"theme":                "Application theme tokens.",
			"pages":                "Legacy page projection. For V3 use protocol.pages.",
			"providers":            "Legacy provider projection. For V3 use protocol.providers.",
			"extensions":           "Legacy extension contributions.",
			"navigation":           "Legacy navigation projection.",
			"components":           "Trusted component definitions supplied by the application.",
			"dependencies":         "Required application identifiers.",
			"runtimeCompatibility": "Runtime compatibility requirement.",
			"pluginVersion":        "Version of the source plugin.",
			"pluginDependencies":   "Required plugin versions.",
			"catalogRevision":      "Runtime catalog revision, when supplied.",
			"previewVersion":       "Preview revision identifier, when supplied.",
			"applicationVersion":   "Runtime application version, when supplied.",
			"definitionDigest":     "Digest of the Runtime definition.",
			"definitionRevision":   "Runtime definition revision, when supplied.",
			"applicationRevision":  "Application lifecycle revision, when supplied.",
			"installationRevision": "Installation lifecycle revision, when supplied.",
		}
		if kind == "page" {
			root = reflect.TypeFor[pageDefinition]()
			g.Title = "Runtime page definition"
			g.Description = "The complete compiled V3 page and the providers referenced by its layout, with route, permission, component tree and action bindings."
			g.Rules = []string{"Identify a page by application ID, page ID and application version.", "Inspect component event bindings and provider mappings to resolve the actual capability contract.", "A compiled page describes structure. Runtime assembly and Gateway validate references and permissions."}
			fields = map[string]string{
				"page":      "V3 page structure, including its route, permission and root node.",
				"providers": "Application provider definitions referenced by this page's layout data.",
			}
		}
		g.Schema, err = inferSchema(root, map[string]reflect.Type{
			"RuntimeNode":        reflect.TypeFor[ar.Node](),
			"RuntimeExpression":  reflect.TypeFor[ar.Expression](),
			"RuntimeField":       reflect.TypeFor[ar.FieldV2](),
			"RuntimeLayoutProps": reflect.TypeFor[ar.LayoutProps](),
			"RuntimePage":        reflect.TypeFor[ar.PageV3](),
			"RuntimeProtocol":    reflect.TypeFor[ar.ApplicationV3](),
			"RuntimeLayout":      reflect.TypeFor[ar.LayoutSpec](),
			"RuntimeComponent":   reflect.TypeFor[ar.ComponentSpec](),
			"RuntimeEvent":       reflect.TypeFor[ar.EventBinding](),
			"RuntimeProvider":    reflect.TypeFor[ar.Provider](),
			"RuntimeData":        reflect.TypeFor[ar.DataSpec](),
		})

	default:
		return schemaGuide{}, fmt.Errorf("unknown schema type %q", kind)
	}
	if err != nil {
		return schemaGuide{}, err
	}
	if kind == "capability" {
		g.Schema.Properties["Output"] = &js.Schema{AnyOf: []*js.Schema{
			{Ref: "#/$defs/CapabilitySchema"}, {Type: "null"},
		}}
	}
	for name, def := range g.Schema.Defs {
		if description, ok := definitionDescriptions[name]; ok {
			def.Description = description.summary
			describeFields(def, description.fields)
		}
	}

	g.Schema.Title, g.Schema.Description = g.Title, g.Description
	g.Schema.Schema = "https://json-schema.org/draft/2020-12/schema"
	describeFields(g.Schema, fields)
	return g, nil
}

func inspectSchema(args []string) (bool, error) {
	if len(args) < 2 || !slices.Contains(schemaKinds, args[0]) || args[1] != "schema" {
		return false, nil
	}
	if len(args) > 3 {
		return true, fmt.Errorf("usage: horizon-cli %s schema [id[@version]]", args[0])
	}
	guide, err := schemaFor(args[0])
	if err != nil {
		return true, err
	}
	if len(args) == 3 {
		c, _, err := loadCatalogKind(args[0])
		if err != nil {
			return true, err
		}
		entry, err := findItem(c.Items, args[0], args[2])
		if err != nil {
			return true, err
		}
		guide.Entry = &entry
	}
	return true, printJSON(guide)
}
