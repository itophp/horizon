package applicationruntime

import (
	"testing"

	"horizon/foundation/kernel/capability/contracts"
)

func TestInvocationDisclosureRequiresTransientStringWrite(t *testing.T) {
	disclosure := &Disclosure{
		Field: "secret", TitleKey: "title", CopyLabelKey: "copy", DismissKey: "close",
		CopySuccessKey: "copied", CopyErrorKey: "error",
	}
	definition := contracts.Definition{
		TransientResult: true,
		Output:          contracts.Object(map[string]*contracts.Schema{"secret": {Type: "string"}, "id": {Type: "integer"}}),
	}
	event := EventBinding{Action: "invoke", Disclosure: disclosure}
	if err := validateInvocationResult(event, definition); err != nil {
		t.Fatal(err)
	}
	disclosure.Field = "id"
	if err := validateInvocationResult(event, definition); err == nil {
		t.Fatal("a non-string field was accepted")
	}
	disclosure.Field = "secret"
	definition.ReadOnly = true
	if err := validateInvocationResult(event, definition); err == nil {
		t.Fatal("a transient read was accepted for disclosure")
	}
	definition.ReadOnly = false
	definition.TransientResult = false
	if err := validateInvocationResult(event, definition); err == nil {
		t.Fatal("a persisted result was accepted for disclosure")
	}
	definition.TransientResult = true
	event.Disclosure = nil
	event.ResultTarget = "_secret"
	if err := validateInvocationResult(event, definition); err == nil {
		t.Fatal("a transient result was allowed in page data")
	}
}

func TestActionFormCannotSaveSubmissionInPageData(t *testing.T) {
	component := &ComponentSpec{
		ID: "form", Kind: "action-form", Version: "1.0",
		Events: []EventBinding{{Event: "submit", Action: "setData", Target: "_data"}},
	}
	if err := validateComponent(component, map[string]bool{"_data": true}); err == nil {
		t.Fatal("the form accepted a binding that could copy the submission to page data")
	}
	component.Events[0].Action = "invoke"
	if err := validateComponent(component, nil); err != nil {
		t.Fatal(err)
	}
	component.Events = append(component.Events, component.Events[0])
	if err := validateComponent(component, nil); err == nil {
		t.Fatal("the form accepted more than one submission binding")
	}
}
