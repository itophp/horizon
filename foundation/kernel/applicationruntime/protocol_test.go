package applicationruntime_test

import (
	ar "horizon/foundation/kernel/applicationruntime"
	"strings"
	"testing"
)

func validApp() ar.ApplicationV3 {
	return ar.ApplicationV3{
		SchemaVersion: "3.0", ID: "test", Version: "1.0.0",
		Source: "test.source", RuntimeCompatibility: "3.0",
		LabelKey: "test:title", Permission: "test:read", Route: "/test",
		Pages: []ar.PageV3{{
			SchemaVersion: "3.0", ID: "test.page", Route: "/test",
			TitleKey: "test:title", Permission: "test:read",
			Root: ar.Node{
				Type: "layout",
				Layout: &ar.LayoutSpec{
					ID: "root", Kind: "grid",
					Props: ar.LayoutProps{Columns: "1fr", Gap: "16px"},
					Data: []ar.DataSpec{
						{ID: "items", Provider: "test.provider"},
					},
					Children: []ar.Node{
						{Type: "component", Component: &ar.ComponentSpec{
							ID: "table", Kind: "data-table", Version: "1.0",
							Data:  "$items",
							Props: map[string]any{"selectable": true},
						}},
					},
				},
			},
		}},
		Providers: []ar.Provider{
			{ID: "test.provider", Capability: ar.Ref("test.capability")},
		},
	}
}

func TestValidAppPasses(t *testing.T) {
	app := validApp()
	if err := ar.ValidateProtocol(app); err != nil {
		t.Fatal("valid app rejected:", err)
	}
}

func TestRejectsWrongSchemaVersion(t *testing.T) {
	app := validApp()
	app.SchemaVersion = "2.0"
	if err := ar.ValidateProtocol(app); err == nil {
		t.Fatal("wrong schema version accepted")
	}
}

func TestRejectsNonLayoutRoot(t *testing.T) {
	app := validApp()
	app.Pages[0].Root = ar.Node{Type: "component", Component: &ar.ComponentSpec{
		ID: "root", Kind: "text", Version: "1.0",
	}}
	if err := ar.ValidateProtocol(app); err == nil {
		t.Fatal("component root accepted")
	}
}

func TestRejectsMultipleComponentsInLayout(t *testing.T) {
	app := validApp()
	app.Pages[0].Root.Layout.Children = []ar.Node{
		{Type: "component", Component: &ar.ComponentSpec{ID: "c1", Kind: "text", Version: "1.0"}},
		{Type: "component", Component: &ar.ComponentSpec{ID: "c2", Kind: "text", Version: "1.0"}},
	}
	if err := ar.ValidateProtocol(app); err == nil {
		t.Fatal("two components in one layout accepted")
	}
}

func TestAllowsMultipleLayoutsInLayout(t *testing.T) {
	app := validApp()
	app.Pages[0].Root.Layout.Children = []ar.Node{
		{Type: "layout", Layout: &ar.LayoutSpec{ID: "l1", Kind: "grid"}},
		{Type: "layout", Layout: &ar.LayoutSpec{ID: "l2", Kind: "grid"}},
		{Type: "layout", Layout: &ar.LayoutSpec{ID: "l3", Kind: "stack"}},
	}
	if err := ar.ValidateProtocol(app); err != nil {
		t.Fatal("multiple layouts rejected:", err)
	}
}

func TestRejectsUndefinedDataReference(t *testing.T) {
	app := validApp()
	app.Pages[0].Root.Layout.Children = []ar.Node{
		{Type: "component", Component: &ar.ComponentSpec{
			ID: "table", Kind: "data-table", Version: "1.0", Data: "$undefined",
		}},
	}
	if err := ar.ValidateProtocol(app); err == nil {
		t.Fatal("undefined data reference accepted")
	}
}

func TestAllowsAncestorDataReference(t *testing.T) {
	app := validApp()
	app.Pages[0].Root.Layout.Children = []ar.Node{
		{Type: "layout", Layout: &ar.LayoutSpec{
			ID: "child", Kind: "grid",
			Children: []ar.Node{
				{Type: "component", Component: &ar.ComponentSpec{
					ID: "table", Kind: "data-table", Version: "1.0", Data: "$items",
				}},
			},
		}},
	}
	if err := ar.ValidateProtocol(app); err != nil {
		t.Fatal("ancestor data reference rejected:", err)
	}
}

func TestRejectsUnknownLayoutKind(t *testing.T) {
	app := validApp()
	app.Pages[0].Root.Layout.Kind = "unknown"
	if err := ar.ValidateProtocol(app); err == nil {
		t.Fatal("unknown layout kind accepted")
	}
}

func TestRejectsUnknownProvider(t *testing.T) {
	app := validApp()
	app.Pages[0].Root.Layout.Data[0].Provider = "missing.provider"
	if err := ar.ValidateProtocol(app); err == nil {
		t.Fatal("unknown provider accepted")
	}
}

func TestRejectsInvalidEventAction(t *testing.T) {
	app := validApp()
	app.Pages[0].Root.Layout.Children = []ar.Node{
		{Type: "component", Component: &ar.ComponentSpec{
			ID: "btn", Kind: "button", Version: "1.0", Data: "$items",
			Events: []ar.EventBinding{{Event: "click", Action: "unknown", Target: "x"}},
		}},
	}
	if err := ar.ValidateProtocol(app); err == nil {
		t.Fatal("invalid event action accepted")
	}
}

func TestRejectsUnregisteredComponentContract(t *testing.T) {
	app := validApp()
	app.Pages[0].Root.Layout.Children[0].Component.Kind = "custom-view"
	if err := ar.ValidateProtocol(app); err == nil {
		t.Fatal("unregistered component accepted")
	}
}

func TestContentSlotsFollowRegisteredComponentContract(t *testing.T) {
	app := validApp()
	component := app.Pages[0].Root.Layout.Children[0].Component
	component.Kind = "dialog"
	component.Props = map[string]any{"titleKey": "runtime:design.preview"}
	component.Content = &ar.LayoutSpec{
		ID:   "dialog-body",
		Kind: "grid",
		Children: []ar.Node{{Type: "component", Component: &ar.ComponentSpec{
			ID: "message", Kind: "text", Version: "1.0",
		}}},
	}
	if err := ar.ValidateProtocol(app); err != nil {
		t.Fatal(err)
	}
	component.Kind = "text"
	if err := ar.ValidateProtocol(app); err == nil {
		t.Fatal("component without a content slot accepted nested layout")
	}
}

func TestRejectsUnsafeOrUnbudgetedStyleStrings(t *testing.T) {
	app := validApp()
	app.Pages[0].Root.Layout.Props.ClassName = "panel panel--primary"
	if err := ar.ValidateProtocol(app); err != nil {
		t.Fatal(err)
	}
	app.Pages[0].Root.Layout.Props.ClassName = "panel\"><script>"
	if err := ar.ValidateProtocol(app); err == nil {
		t.Fatal("unsafe layout class accepted")
	}
	app.Pages[0].Root.Layout.Props.ClassName = ""
	app.Pages[0].Root.Layout.Children[0].Component.Props["style"] = map[string]any{"color": "red"}
	if err := ar.ValidateProtocol(app); err == nil {
		t.Fatal("inline component style accepted")
	}
	delete(app.Pages[0].Root.Layout.Children[0].Component.Props, "style")
	app.Pages[0].Root.Layout.Props.Columns = strings.Repeat("1fr ", 501)
	if err := ar.ValidateProtocol(app); err == nil {
		t.Fatal("unbudgeted style string accepted")
	}
}

func TestRejectsMaxDepth(t *testing.T) {
	app := validApp()
	// Build a chain deeper than maxDepth (10)
	root := app.Pages[0].Root.Layout
	current := root
	for i := 0; i < 15; i++ {
		child := &ar.Node{Type: "layout", Layout: &ar.LayoutSpec{
			ID: "deep", Kind: "grid",
		}}
		current.Children = []ar.Node{*child}
		current = child.Layout
	}
	if err := ar.ValidateProtocol(app); err == nil {
		t.Fatal("max depth exceeded accepted")
	}
}

func TestValidNestedLayoutWithComponentPerLevel(t *testing.T) {
	app := validApp()
	app.Pages[0].Root.Layout.Children = []ar.Node{
		{Type: "layout", Layout: &ar.LayoutSpec{
			ID: "header", Kind: "grid", Props: ar.LayoutProps{Columns: "auto 1fr", Gap: "8px"},
			Children: []ar.Node{
				{Type: "component", Component: &ar.ComponentSpec{ID: "title", Kind: "text", Version: "1.0"}},
			},
		}},
		{Type: "layout", Layout: &ar.LayoutSpec{
			ID: "body", Kind: "grid", Props: ar.LayoutProps{Columns: "1fr 3fr", Gap: "16px"},
			Children: []ar.Node{
				{Type: "layout", Layout: &ar.LayoutSpec{ID: "sidebar", Kind: "sidebar"}},
				{Type: "layout", Layout: &ar.LayoutSpec{
					ID: "main", Kind: "grid",
					Children: []ar.Node{
						{Type: "component", Component: &ar.ComponentSpec{
							ID: "table", Kind: "data-table", Version: "1.0", Data: "$items",
						}},
					},
				}},
			},
		}},
	}
	if err := ar.ValidateProtocol(app); err != nil {
		t.Fatal("valid nested layout rejected:", err)
	}
}
