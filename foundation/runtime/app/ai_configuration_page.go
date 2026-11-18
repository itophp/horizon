package app

import (
	"horizon/foundation/ai/configuration"
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/system/governance"
	"strings"
)

const aiConfigurationData = "_aiConfiguration"

func activeAISettingsApplication(version string) ar.ApplicationV2 {
	const id = "ai-settings"
	const route = "/ai-settings"
	title, subtitle := "governance:aiSettings.title", "governance:aiSettings.hint"
	editAllowed := permits(configuration.UpdatePermission)
	save := ar.EventBinding{
		Event: "submit", Action: "invoke", Target: "ai.config.save@1.0.0", When: &editAllowed,
		Payload: objectValue(
			mapping("baseRevision", fieldPath("data."+aiConfigurationData+".revision")),
			mapping("changes", fieldPath("event.changes")),
		),
	}
	var tabs []viewProps
	var panels []ar.Node
	for _, tab := range []string{"default", "models", "routes"} {
		tabs = append(tabs, viewProps{"id": tab, "labelKey": "governance:aiSettings.tabs." + tab})
		panel := componentNode(id+"-"+tab, "ai-model-settings", "$"+aiConfigurationData, viewProps{"view": tab}, save)
		componentOf(panel).Bindings = []ar.Mapping{mapping("disabled", operation("not", editAllowed))}
		panels = append(panels, platformTab(tab, panel))
	}
	props, _ := settingsChangeFormProps(func(definition governance.Definition) bool {
		if version == "1.0.1" && governance.IsBailianEmbeddingSetting(definition.Key) {
			return false
		}
		if version != "1.0.1" && strings.HasPrefix(definition.Key, "knowledge.embedding.") &&
			definition.Key != governance.KnowledgeEmbeddingProfileKey {
			return false
		}
		return governance.IsAISetting(definition.Key) && !governance.IsAIModelSetting(definition.Key)
	}, "governance:save")
	general := componentNode("ai-services-editor", "change-list-form", "$"+aiConfigurationData, props, save)
	componentOf(general).Bindings = []ar.Mapping{mapping("disabled", operation("not", editAllowed))}
	if version == "1.0.3" {
		for _, field := range props["fields"].([]viewProps) {
			if field["key"] == governance.KnowledgeEmbeddingProfileKey {
				delete(field, "options")
				field["optionRows"] = viewProps{"source": "embeddingModels", "valuePath": "id", "labelPath": "label"}
			}
		}
		componentOf(general).Bindings = append(componentOf(general).Bindings,
			mapping("optionData", objectValue(mapping("embeddingModels", fieldPath("data."+aiConfigurationData+".embeddingModels")))),
		)
	}
	tabs = append(tabs, viewProps{"id": "general", "labelKey": "governance:aiSettings.tabs.general", "icon": "sliders"})
	panels = append(panels, platformTab("general", general))

	app := platformPageVersion(id, route, title, subtitle, version, []ar.Provider{{
		ID: "ai-configuration", Capability: ar.Ref("ai.config.read"), KeyField: "revision", Pagination: "none",
	}}, platformTabs(id+"-tabs", viewProps{"param": "tab", "tabs": tabs}, panels...))
	app.Permission, app.Protocol.Permission = configuration.ReadPermission, configuration.ReadPermission
	page := &app.Protocol.Pages[0]
	page.Permission = configuration.ReadPermission
	page.Root.Layout.Props.ClassName = "ai-settings-page"
	page.Root.Layout.Data = []ar.DataSpec{{ID: aiConfigurationData, Provider: "ai-configuration", NonBlocking: true}}
	return app
}
