package app

import (
	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/system/governance"
)

func legacyAISettingsApplication() ar.ApplicationV2 {
	const id = "ai-settings"
	const route = "/ai-settings"
	const permission = "settings:read"
	providers := workspaceSettingsProviders()[:2]
	app := platformPage(id, route, "governance:aiSettings.title", "governance:aiSettings.hint", providers)
	app.Permission, app.Protocol.Permission = permission, permission
	app.Protocol.Pages = []ar.PageV3{aiSettingsPage(id+".home", true)}
	return app
}

func aiSettingsPage(id string, standalone bool) ar.PageV3 {
	editAllowed := permits("settings:update")
	tabs := []viewProps{}
	panels := []ar.Node{}
	for _, tab := range []string{"default", "models", "routes"} {
		tabs = append(tabs, viewProps{"id": tab, "labelKey": "governance:aiSettings.tabs." + tab})
		form := componentNode("ai-settings-"+tab, "ai-model-settings", "$"+workspaceSettingsData,
			viewProps{"view": tab}, ar.EventBinding{
				Event: "submit", Action: "invoke", Target: "settings.draft.create@1.0.0", When: &editAllowed,
				Payload: objectValue(
					mapping("target", workspaceTarget()),
					mapping("baseRevision", fieldPath("data."+workspaceSettingsData+".revision")),
					mapping("changes", fieldPath("event.changes")),
					mapping("reason", fieldPath("event.reason")),
				),
			})
		componentOf(form).Bindings = []ar.Mapping{mapping("disabled", operation("not", editAllowed))}
		panels = append(panels, platformTab(tab, form))
	}
	releases := []ar.Node{
		workspaceDrafts(), workspaceHistory(), workspaceScheduleQueue(),
		workspaceActionDialog("publish"), workspaceActionDialog("rollback"), workspaceActionDialog("schedule"),
	}
	if standalone {
		tabs = append(tabs,
			viewProps{"id": "general", "labelKey": "governance:aiSettings.tabs.general", "icon": "sliders"},
			viewProps{"id": "releases", "labelKey": "governance:aiSettings.tabs.releases", "icon": "upload"},
		)
		general := workspaceSettingsForm(same("query.tab", "general"), func(definition governance.Definition) bool {
			return governance.IsAISetting(definition.Key) && !governance.IsAIModelSetting(definition.Key) &&
				!governance.IsBailianEmbeddingSetting(definition.Key)
		})
		panels = append(panels, platformTab("general", general), platformTab("releases",
			append([]ar.Node{componentNode("ai-release-hint", "text", "", viewProps{"textKey": "governance:aiSettings.releaseHint", "className": "muted-text"})}, releases...)...,
		))
		releases = nil
	}
	content := []ar.Node{
		componentNode("ai-settings-heading", "page-heading", "", viewProps{"titleKey": "governance:aiSettings.title", "subtitleKey": "governance:aiSettings.hint"}),
		platformTabs("ai-settings-tabs", viewProps{"param": "tab", "tabs": tabs}, panels...),
	}
	layout := ar.LayoutProps{Gap: "16px", Width: "full"}
	if standalone {
		layout.ClassName = "ai-settings-page"
	}
	root := layoutNode("ai-settings-root", layout, append(content, releases...)...)
	root.Layout.Data = []ar.DataSpec{
		{ID: workspaceSettingsData, Provider: "workspace-settings", NonBlocking: true, PollMs: 5000},
		{ID: workspaceSchedulesData, Provider: "workspace-schedules", NonBlocking: true, PollMs: 5000},
		{ID: workspaceActionData},
	}
	return ar.PageV3{
		SchemaVersion: "3.0", ID: id, Route: "/ai-settings",
		TitleKey: "governance:aiSettings.title", SubtitleKey: "governance:aiSettings.hint",
		Permission: "settings:read", Root: root,
	}
}
