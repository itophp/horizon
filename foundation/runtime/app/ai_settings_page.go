package app

import ar "horizon/foundation/kernel/applicationruntime"

func aiSettingsPage() ar.PageV3 {
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
	root := layoutNode("ai-settings-root", ar.LayoutProps{Gap: "16px", Width: "full"},
		componentNode("ai-settings-heading", "page-heading", "", viewProps{"titleKey": "governance:aiSettings.title", "subtitleKey": "governance:aiSettings.hint"}),
		platformTabs("ai-settings-tabs", viewProps{"param": "tab", "tabs": tabs}, panels...),
		workspaceDrafts(), workspaceHistory(), workspaceScheduleQueue(),
		workspaceActionDialog("publish"), workspaceActionDialog("rollback"), workspaceActionDialog("schedule"),
	)
	root.Layout.Data = []ar.DataSpec{
		{ID: workspaceSettingsData, Provider: "workspace-settings", NonBlocking: true, PollMs: 5000},
		{ID: workspaceSchedulesData, Provider: "workspace-schedules", NonBlocking: true, PollMs: 5000},
		{ID: workspaceActionData},
	}
	return ar.PageV3{
		SchemaVersion: "3.0", ID: "system-settings.ai", Route: "/ai-settings",
		TitleKey: "governance:aiSettings.title", SubtitleKey: "governance:aiSettings.hint",
		Permission: "settings:read", Root: root,
	}
}
