package applicationruntime

import (
	"encoding/json"
	"fmt"
	"slices"

	"horizon/foundation/kernel/capability/contracts"
)

type shadcnContract struct {
	name, description string
	props             *contracts.Schema
	events            []string
	content, data     bool
}

var shadcnContracts = func() map[string]shadcnContract {
	str := &contracts.Schema{Type: "string", MaxLength: new(160)}
	boolean := &contracts.Schema{Type: "boolean"}
	num := &contracts.Schema{Type: "number"}
	orientation := &contracts.Schema{Type: "string", Enum: []string{"horizontal", "vertical"}}
	options := &contracts.Schema{Type: "array", MaxItems: new(500), Items: contracts.Object(
		map[string]*contracts.Schema{"value": str, "labelKey": str, "disabled": boolean}, "value", "labelKey",
	)}
	result := map[string]shadcnContract{}
	add := func(name, description string, content, data bool, event string, properties map[string]*contracts.Schema, required ...string) {
		events := []string{}
		if event != "" {
			events = append(events, event)
		}
		result[name] = shadcnContract{name, description, contracts.Object(properties, required...), events, content, data}
	}
	label := func() map[string]*contracts.Schema { return map[string]*contracts.Schema{"labelKey": str} }
	titles := func() map[string]*contracts.Schema {
		return map[string]*contracts.Schema{"titleKey": str, "descriptionKey": str}
	}
	choice := func() map[string]*contracts.Schema {
		return map[string]*contracts.Schema{"labelKey": str, "options": options, "disabled": boolean}
	}
	input := func(value *contracts.Schema) map[string]*contracts.Schema {
		return map[string]*contracts.Schema{"labelKey": str, "id": str, "disabled": boolean, "value": value}
	}
	add("accordion", "Expandable section. Set group to compose accordion-item children. change emits the expanded item values.", true, false, "change",
		map[string]*contracts.Schema{"labelKey": str, "open": boolean, "group": boolean, "multiple": boolean, "expanded": {Type: "array", Items: str, MaxItems: new(100)}}, "labelKey")
	add("accordion-item", "One titled accordion section inside a grouped accordion.", true, false, "",
		map[string]*contracts.Schema{"value": str, "labelKey": str, "disabled": boolean}, "labelKey", "value")
	add("collapsible", "Expandable section with bindable open state. openChange emits the new boolean.", true, false, "openChange",
		map[string]*contracts.Schema{"labelKey": str, "open": boolean, "disabled": boolean}, "labelKey")
	for _, name := range []string{"alert", "empty", "item"} {
		props := titles()
		props["sections"] = boolean
		if name == "alert" {
			props["variant"] = &contracts.Schema{Type: "string", Enum: []string{"default", "destructive"}}
		}
		add(name, "Displays a translated title and optional description and content.", true, false, "", props, "titleKey")
	}
	add("aspect-ratio", "Maintains a fixed width to height ratio.", true, false, "", map[string]*contracts.Schema{"ratio": num})
	add("avatar", "Avatar with fallback initials and an optional same-origin image path.", false, false, "", map[string]*contracts.Schema{
		"labelKey": str, "initials": str, "src": {Type: "string", MaxLength: new(2048)},
		"size": {Type: "string", Enum: []string{"sm", "default", "lg"}},
	}, "labelKey")
	add("avatar-group", "Overlapping avatars in the content layout.", true, false, "", label(), "labelKey")
	add("badge", "Displays a short translated status label.", false, false, "", map[string]*contracts.Schema{"labelKey": str, "variant": {Type: "string", Enum: []string{"secondary", "outline", "default", "destructive", "ghost", "link"}}}, "labelKey")
	add("breadcrumb", "Ancestor trail. select emits the selected ancestor value.", false, false, "select", choice(), "labelKey", "options")
	add("button-group", "Groups adjacent button elements in a content layout.", true, false, "", map[string]*contracts.Schema{"orientation": orientation})
	add("carousel", "Scrollable gallery. startIndex and change use zero-based slide indices. autoplayMs sets the rotation interval, with a pause control.", true, false, "change", map[string]*contracts.Schema{
		"labelKey": str, "orientation": orientation, "loop": boolean, "controls": boolean,
		"startIndex": {Type: "integer", Minimum: new(0.0), Maximum: new(499.0)},
		"autoplayMs": {Type: "integer", Minimum: new(1000.0), Maximum: new(60000.0)},
	}, "labelKey")
	add("carousel-slide", "One content layout inside a carousel.", true, false, "", nil)
	add("field", "Label, description and validation error for a control. for references the control id. Description and error ids append -description and -field-error.", true, false, "", map[string]*contracts.Schema{
		"labelKey": str, "descriptionKey": str, "for": str, "invalid": boolean, "errorKey": str,
		"orientation": {Type: "string", Enum: []string{"vertical", "horizontal", "responsive"}},
	}, "labelKey")
	add("kbd", "Translated keyboard shortcut hint.", false, false, "", label(), "labelKey")
	add("label", "Label for a control with a matching id.", false, false, "", map[string]*contracts.Schema{"labelKey": str, "for": str}, "labelKey", "for")
	add("progress", "Progress over a configurable range. Omit the value for indeterminate progress.", false, true, "", map[string]*contracts.Schema{"labelKey": str, "value": num, "min": num, "max": num, "showValue": boolean, "format": {Type: "string", Enum: []string{"percent", "number"}}}, "labelKey")
	add("resizable", "Layout of resizable-panel elements separated by resizable-handle elements.", true, false, "", map[string]*contracts.Schema{"orientation": orientation})
	panelSize := &contracts.Schema{Type: "number", Minimum: new(0.0), Maximum: new(100.0)}
	add("resizable-panel", "Panel inside a resizable group. Sizes are percentages of the group.", true, false, "",
		map[string]*contracts.Schema{"minSize": panelSize, "maxSize": panelSize, "defaultSize": panelSize, "disabled": boolean})
	add("resizable-handle", "Keyboard and pointer resizing handle between two resizable panels.", false, false, "", nil)
	add("scroll-area", "Scrollable region with vertical, horizontal, or both scrollbars. Height is expressed in pixels.", true, false, "",
		map[string]*contracts.Schema{"height": num, "labelKey": str, "axes": {Type: "string", Enum: []string{"vertical", "horizontal", "both"}}, "keepScrollbars": boolean})
	add("separator", "Visual divider.", false, false, "", map[string]*contracts.Schema{"orientation": orientation})
	add("skeleton", "Decorative loading placeholder.", false, false, "", map[string]*contracts.Schema{"shape": {Type: "string", Enum: []string{"line", "circle", "block"}}})
	add("spinner", "Accessible loading indicator.", false, false, "", label())
	add("table", "Read-only table. data supplies rows and options supplies column keys and labels.", false, true, "", map[string]*contracts.Schema{"labelKey": str, "options": options}, "options")
	add("typography", "Translated text with paragraph, heading or caption styling.", false, false, "",
		map[string]*contracts.Schema{"labelKey": str, "variant": {Type: "string", Enum: []string{"heading", "caption", "paragraph", "h1", "h2", "h3", "h4", "h5", "h6", "quote", "code", "ul", "ol"}}, "itemKeys": {Type: "array", Items: str, MaxItems: new(100)}}, "labelKey")
	for _, name := range []string{"checkbox", "switch", "toggle"} {
		add(name, "Boolean control. change emits a boolean.", false, true, "change", input(boolean), "labelKey")
	}
	for _, name := range []string{"input", "textarea", "input-group"} {
		props := input(str)
		props["placeholderKey"], props["errorKey"], props["describedBy"] = str, str, str
		props["invalid"], props["required"] = boolean, boolean
		length := &contracts.Schema{Type: "integer", Minimum: new(0.0), Maximum: new(65536.0)}
		props["minLength"], props["maxLength"] = length, length
		if name == "textarea" {
			props["rows"] = &contracts.Schema{Type: "integer", Minimum: new(1.0), Maximum: new(50.0)}
		} else {
			props["type"] = &contracts.Schema{Type: "string", Enum: []string{"text", "email", "url", "tel", "search", "number"}}
		}
		if name == "input-group" {
			props["descriptionKey"] = str
		}
		add(name, "Text control. change emits a string.", false, true, "change", props, "labelKey")
	}
	add("input-otp", "One-time code. complete emits the code only after all digits are entered.", false, false, "complete",
		map[string]*contracts.Schema{"labelKey": str, "id": str, "disabled": boolean, "length": {Type: "integer", Minimum: new(1.0), Maximum: new(12.0)}}, "labelKey")
	for _, name := range []string{"combobox", "native-select", "radio-group", "select"} {
		props := input(str)
		props["options"], props["placeholderKey"] = options, str
		if name == "combobox" {
			props["multiple"] = boolean
			props["values"] = &contracts.Schema{Type: "array", Items: str, MaxItems: new(500)}
			add(name, "Fixed enumeration control. Multiple mode emits a value array and displays selected chips. Use the authorized search picker for business objects.", false, true, "change", props, "labelKey", "options")
			continue
		}
		add(name, "Fixed enumeration control. change emits the selected value. Use an authorized search picker for business objects.", false, true, "change", props, "labelKey", "options")
	}
	toggleGroup := input(str)
	toggleGroup["options"], toggleGroup["multiple"] = options, boolean
	toggleGroup["values"] = &contracts.Schema{Type: "array", Items: str, MaxItems: new(500)}
	add("toggle-group", "Fixed enumeration controls. change emits a value or, in multiple mode, a value array.", false, true, "change", toggleGroup, "labelKey", "options")
	props := input(num)
	props["min"], props["max"], props["step"] = num, num, num
	props["mode"] = &contracts.Schema{Type: "string", Enum: []string{"single", "range"}}
	props["orientation"] = orientation
	props["range"] = &contracts.Schema{Type: "array", Items: num, MinItems: new(2), MaxItems: new(2)}
	add("slider", "Numeric control. change emits a number or a two-value range when the gesture is committed.", false, true, "change", props, "labelKey")
	command := choice()
	command["mode"] = &contracts.Schema{Type: "string", Enum: []string{"inline", "dialog"}}
	command["open"], command["placeholderKey"], command["titleKey"], command["descriptionKey"] = boolean, str, str, str
	commandOptions := *options
	commandOptions.Items = contracts.Object(map[string]*contracts.Schema{
		"value": str, "labelKey": str, "disabled": boolean, "groupKey": str, "shortcutKey": str,
	}, "value", "labelKey")
	command["options"] = &commandOptions
	add("command", "Inline or dialog command palette with optional groups and shortcut hints. select emits the command value. openChange emits the dialog state.", false, false, "select", command, "labelKey", "options")
	commandContract := result["command"]
	commandContract.events = append(commandContract.events, "openChange")
	result["command"] = commandContract
	menuItems := options.Items
	for range 3 {
		menuItems = contracts.Object(map[string]*contracts.Schema{
			"value": str, "labelKey": str, "disabled": boolean, "checked": boolean, "selected": str,
			"kind":    {Type: "string", Enum: []string{"action", "checkbox", "radio", "submenu"}},
			"options": {Type: "array", Items: menuItems, MaxItems: new(20)},
		}, "value", "labelKey")
	}
	menuOptions := &contracts.Schema{Type: "array", Items: menuItems, MaxItems: new(500)}
	for _, name := range []string{"context-menu", "dropdown-menu"} {
		props := choice()
		props["options"] = menuOptions
		add(name, "Menu with actions, checkboxes, radio groups and submenus. select emits a string, {value, checked}, or {group, value}. Use options for nested items.", name == "context-menu", false, "select", props, "labelKey", "options")
	}
	menuProps := choice()
	menuProps["options"] = menuOptions
	menuProps["menus"] = &contracts.Schema{Type: "array", MinItems: new(1), MaxItems: new(20), Items: contracts.Object(
		map[string]*contracts.Schema{
			"value": str, "labelKey": str, "disabled": boolean,
			"options": {Type: "array", Items: menuItems, MaxItems: new(50)},
		}, "value", "labelKey", "options",
	)}
	add("menubar", "Menu bar. Set options for one menu or menus for multiple groups. select emits a string for an action, {value, checked} for a checkbox, or {group, value} for radio. Grouped menus include menu in the event object.", false, false, "select", menuProps, "labelKey")
	add("navigation-menu-item", "One navigation trigger and its content inside a grouped navigation-menu.", true, false, "",
		map[string]*contracts.Schema{"labelKey": str, "disabled": boolean}, "labelKey")
	for _, name := range []string{"alert-dialog", "drawer", "popover", "sheet", "toast", "sonner", "tooltip", "hover-card", "navigation-menu"} {
		props := titles()
		props["labelKey"] = str
		required := []string{"labelKey"}
		event := ""
		content := name != "toast" && name != "sonner" && name != "tooltip"
		if slices.Contains([]string{"drawer", "popover", "sheet"}, name) {
			props["side"] = &contracts.Schema{Type: "string", Enum: []string{"top", "right", "bottom", "left"}}
			props["open"], props["disabled"] = boolean, boolean
			event = "openChange"
		}
		if name == "alert-dialog" {
			props["confirmKey"], props["cancelKey"] = str, str
			props["open"], props["hideTrigger"], props["disabled"] = boolean, boolean, boolean
			required = append(required, "titleKey", "confirmKey")
			event = "confirm"
		}
		if slices.Contains([]string{"drawer", "sheet", "toast", "sonner"}, name) {
			required = append(required, "titleKey")
		}
		if name == "toast" || name == "sonner" {
			props["mode"] = &contracts.Schema{Type: "string", Enum: []string{"local", "action"}}
			props["successKey"], props["errorKey"], props["disabled"] = str, str, boolean
			event = "click"
		}
		if name == "navigation-menu" {
			props["group"] = boolean
		}
		if name == "tooltip" || name == "hover-card" {
			props["side"] = &contracts.Schema{Type: "string", Enum: []string{"top", "right", "bottom", "left"}}
			delay := &contracts.Schema{Type: "integer", Minimum: new(0.0), Maximum: new(5000.0)}
			props["delay"], props["closeDelay"] = delay, delay
		}
		if name == "tooltip" {
			required = append(required, "descriptionKey")
		}
		description := "Translated overlay or local feedback triggered by the user."
		if name == "alert-dialog" {
			description = "Confirmation dialog with an optional bound open state. Emits confirm for confirmation, cancel for dismissal, and openChange with the new state."
		}
		add(name, description, content, false, event, props, required...)
		if name == "alert-dialog" {
			contract := result[name]
			contract.events = append(contract.events, "cancel", "openChange")
			result[name] = contract
		}
	}
	props = choice()
	props["value"], props["orientation"] = str, orientation
	props["activation"] = &contracts.Schema{Type: "string", Enum: []string{"manual", "automatic"}}
	props["variant"] = &contracts.Schema{Type: "string", Enum: []string{"default", "line"}}
	add("tabs", "Tab choices and a content layout with layout-tab-panel elements. change emits a tab value.", true, true, "change", props, "labelKey", "options")
	for _, name := range []string{"calendar", "date-picker"} {
		props := input(str)
		props["placeholderKey"] = str
		props["mode"] = &contracts.Schema{Type: "string", Enum: []string{"single", "multiple", "range"}}
		props["dates"] = &contracts.Schema{Type: "array", Items: str, MaxItems: new(366)}
		props["range"] = contracts.Object(map[string]*contracts.Schema{"from": str, "to": str}, "from")
		props["minDate"], props["maxDate"], props["clearable"] = str, str, boolean
		props["months"] = &contracts.Schema{Type: "integer", Minimum: new(1.0), Maximum: new(3.0)}
		add(name, "Date selection in YYYY-MM-DD. Use value for single, dates for multiple or range with from and optional to for range. change emits a date string, an array of dates, a range object, or null when cleared.", false, true, "change", props, "labelKey")
	}
	add("chart", "Chart with numeric series and category labels from data. Configure series with field and labelKey, or use valueField and labelKey for one series. Bar and area charts can stack. Pie, donut and radial-bar use the first series and positive values. Radar supports multiple series.", false, true, "",
		map[string]*contracts.Schema{
			"titleKey": str, "labelKey": str, "labelField": str, "valueField": str,
			"variant": {Type: "string", Enum: []string{"bar", "line", "area", "pie", "donut", "radar", "radial-bar"}},
			"series": {Type: "array", MinItems: new(1), MaxItems: new(10), Items: contracts.Object(
				map[string]*contracts.Schema{"field": str, "labelKey": str}, "field", "labelKey",
			)},
			"stacked": boolean, "legend": boolean,
		}, "titleKey", "labelKey")
	add("pagination", "Pagination links preserve other URL query parameters. Set numbered to show first, last and nearby pages.", false, false, "",
		map[string]*contracts.Schema{"labelKey": str, "pages": {Type: "integer", Minimum: new(1.0)}, "param": str, "disabled": boolean, "numbered": boolean}, "labelKey", "pages")
	add("sidebar", "Collapsible local sidebar containing a content layout.", true, false, "",
		map[string]*contracts.Schema{"titleKey": str, "open": boolean, "sections": boolean}, "titleKey")
	for _, name := range []string{"sidebar-header", "sidebar-footer", "sidebar-content", "sidebar-group"} {
		add(name, "Content slot for a structured sidebar.", true, false, "", label())
	}
	for _, region := range []struct{ name, description string }{
		{"card-footer", "Footer region for actions or supporting content inside a card."},
		{"alert-action", "Action region inside an alert."},
		{"empty-media", "Media region inside an empty state."},
		{"item-media", "Media region inside an item."},
		{"item-actions", "Action region inside an item."},
		{"item-footer", "Footer region inside an item."},
	} {
		var props map[string]*contracts.Schema
		switch region.name {
		case "empty-media":
			props = map[string]*contracts.Schema{"variant": contracts.Enum("default", "icon")}
		case "item-media":
			props = map[string]*contracts.Schema{"variant": contracts.Enum("default", "icon", "image")}
		}
		add(region.name, region.description, true, false, "", props)
	}
	return result
}()

func appendShadcnUIContracts(result []UIContract) []UIContract {
	names := make([]string, 0, len(shadcnContracts))
	for name := range shadcnContracts {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		item := shadcnContracts[name]
		slots := []string{}
		if item.content {
			slots = append(slots, "content")
		}
		binding := ""
		if item.data {
			binding = "data"
		}
		props := map[string]any{}
		for _, key := range item.props.Required {
			switch item.props.Properties[key].Type {
			case "array":
				props[key] = []any{map[string]any{"value": "example", "labelKey": "runtime:components.loading"}}
			case "integer":
				props[key] = 1
			default:
				if key == "value" || key == "for" {
					props[key] = "example"
				} else {
					props[key] = "runtime:components.loading"
				}
			}
		}
		switch name {
		case "menubar":
			props["menus"] = []any{
				map[string]any{"value": "navigation", "labelKey": "runtime:components.previousPage", "options": []any{
					map[string]any{"value": "next", "labelKey": "runtime:components.nextPage"},
				}},
			}
		case "chart":
			props["series"] = []any{
				map[string]any{"field": "planned", "labelKey": "runtime:components.previousPage"},
				map[string]any{"field": "actual", "labelKey": "runtime:components.nextPage"},
			}
			props["variant"] = "bar"
		}
		example, _ := json.Marshal(map[string]any{
			"type": "component", "component": map[string]any{"id": name, "kind": name, "version": "1.0", "props": props},
		})
		result = append(result, UIContract{
			Name: name, Version: "1.0", Category: "element", Description: item.description,
			PropsSchema: string(item.props.JSON()), Slots: slots, Events: item.events, Binding: binding,
			ReadOnly: "Suppress action events and editing; allow local inspection.", Example: string(example),
		})
	}
	return result
}

func validateShadcnComponent(comp *ComponentSpec) error {
	item, ok := shadcnContracts[comp.Kind]
	if !ok || comp.Version != "1.0" {
		return nil
	}
	// Validate static properties independently of values resolved by bindings.
	props := make(map[string]any, len(comp.Props))
	for key, value := range comp.Props {
		props[key] = value
	}
	bound := map[string]bool{}
	for _, binding := range comp.Bindings {
		if item.props.Properties[binding.Target] == nil {
			return fmt.Errorf("schema_invalid: component %s unknown binding %s", comp.ID, binding.Target)
		}
		bound[binding.Target] = true
	}
	schema := *item.props
	schema.Required = nil
	for _, key := range item.props.Required {
		if !bound[key] {
			schema.Required = append(schema.Required, key)
		}
	}
	// Go contributions and decoded JSON may use different concrete slice types.
	raw, err := json.Marshal(props)
	if err != nil || json.Unmarshal(raw, &props) != nil || schema.Validate(props) != nil {
		return fmt.Errorf("schema_invalid: component %s props do not match %s@%s", comp.ID, comp.Kind, comp.Version)
	}
	if comp.Data != "" && !item.data {
		return fmt.Errorf("schema_invalid: component %s does not accept data", comp.ID)
	}
	for _, event := range comp.Events {
		if comp.Kind == "input-otp" && (event.Action != "invoke" || len(comp.Events) > 1 || event.ResultTarget != "" || event.Download != nil) {
			return fmt.Errorf("schema_invalid: component %s requires one capability invocation without a persisted result target for an OTP", comp.ID)
		}
		if !slices.Contains(item.events, event.Event) {
			return fmt.Errorf("schema_invalid: component %s does not emit %s", comp.ID, event.Event)
		}
	}
	return nil
}
