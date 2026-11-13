package applicationruntime

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"horizon/foundation/kernel/capability/contracts"
)

var runtimeDownloadFilename = regexp.MustCompile(`^[a-zA-Z0-9_.-]{1,100}\.(json|csv)$`)

func validDownload(download DownloadResult, output *contracts.Schema) bool {
	if !runtimeDownloadFilename.MatchString(download.Filename) {
		return false
	}
	if download.ContentField == "" {
		return download.FilenameField == "" && !strings.HasSuffix(download.Filename, ".csv")
	}
	if output == nil {
		return false
	}
	content := output.Properties[download.ContentField]
	filename := output.Properties[download.FilenameField]
	return content != nil && content.Type == "string" &&
		(download.FilenameField == "" || (filename != nil && filename.Type == "string"))
}

var proofOperations = map[string]string{
	"settings.draft.publish@1.0.0":      "settings.publish",
	"settings.schedule.create@1.0.0":    "settings.schedule",
	"settings.history.rollback@1.0.0":   "settings.rollback",
	"dictionary.history.rollback@1.0.0": "dictionaries.rollback",
}

func validateInvocationReauthentication(component *ComponentSpec, event *EventBinding, definitions map[string]contracts.Definition) error {
	auth := event.Reauthenticate
	if auth == nil {
		return nil
	}
	operation, allowed := proofOperations[event.Target]
	verifier := definitions[auth.Capability.Name+"@"+auth.Capability.Version]
	if component.Kind != "action-form" || event.Event != "submit" ||
		!allowed || operation != auth.Operation || auth.Capability != Ref("settings.password.verify") ||
		!verifier.TransientResult || !verifier.ReadOnly ||
		event.Result != nil || event.ResultTarget != "" || event.Disclosure != nil || event.Download != nil {
		return fmt.Errorf("invalid reauthentication binding")
	}
	if event.Payload.Kind != "object" || slices.ContainsFunc(event.Payload.Fields, func(mapping Mapping) bool {
		return mapping.Target == "proof" || mapping.Target == "password"
	}) {
		return fmt.Errorf("reauthentication inputs must leave proof and password unbound")
	}
	if auth.When != nil {
		return ValidateExpression(*auth.When, 0)
	}
	return nil
}
