package applicationruntime

import (
	"testing"

	"horizon/applications/projects"
	ar "horizon/foundation/kernel/applicationruntime"
)

func TestPublishedProjectImportReceivesDeploymentTemplate(t *testing.T) {
	baseline := projects.RuntimeApplication()
	module := &Module{Versions: []ar.ApplicationV2{baseline}}
	published := ar.Canonical(baseline)
	var component *ar.ComponentSpec
	for _, page := range published.Protocol.Pages {
		if page.ID == "project.import" {
			component, _ = ar.FindAIComponent(page.Root, "project-batch-import")
		}
	}
	if component == nil {
		t.Fatal("project import component not found")
	}
	delete(component.Props, "templatePath")
	published.PluginVersion = "0.0.0"
	module.supplyPublishedImportTemplates(&published)
	if _, exists := component.Props["templatePath"]; exists {
		t.Fatal("template supplied from a different plugin version")
	}
	published.PluginVersion = baseline.PluginVersion
	module.supplyPublishedImportTemplates(&published)
	if got := component.Props["templatePath"]; got != "/templates/projects-import.xlsx" {
		t.Fatalf("legacy snapshot template: %v", got)
	}
	if err := ar.ValidateProtocol(*published.Protocol); err != nil {
		t.Fatal(err)
	}
	component.Props["templatePath"] = "/templates/custom.xlsx"
	module.supplyPublishedImportTemplates(&published)
	if got := component.Props["templatePath"]; got != "/templates/custom.xlsx" {
		t.Fatalf("custom template was replaced: %v", got)
	}
}
