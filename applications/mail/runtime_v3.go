package mail

import ar "horizon/foundation/kernel/applicationruntime"

func componentLayout(id string, component *ar.ComponentSpec) ar.Node {
	return ar.Node{Type: "layout", Layout: &ar.LayoutSpec{
		ID: id, Kind: "grid", Props: ar.LayoutProps{Columns: "minmax(0, 1fr)"},
		Children: []ar.Node{{Type: "component", Component: component}},
	}}
}

func mailProvider(id string, capability string, input ...ar.Mapping) ar.Provider {
	return ar.Provider{
		ID: id, Capability: ar.Ref(capability), KeyField: "id", Pagination: "server",
		PageSize: 50, Tags: []string{"mail"}, Input: input,
	}
}

func mailPage(id, route, titleKey, permission, dataID, providerID, mode string) ar.PageV3 {
	component := &ar.ComponentSpec{
		ID: id + "-workspace", Kind: "mail-workspace", Version: "1.0",
		Data: "$" + dataID, Props: map[string]any{"mode": mode, "titleKey": titleKey},
	}
	return ar.PageV3{
		SchemaVersion: "3.0", ID: id, Route: route, TitleKey: titleKey,
		SubtitleKey: "mail:description", Permission: permission,
		Root: ar.Node{Type: "layout", Layout: &ar.LayoutSpec{
			ID: id + "-root", Kind: "stack", Props: ar.LayoutProps{Gap: "16px", Width: "full"},
			Data: []ar.DataSpec{{ID: dataID, Provider: providerID}}, Children: []ar.Node{componentLayout(id, component)},
		}},
	}
}

func RuntimeApplication() ar.ApplicationV2 {
	inbox := mailProvider("mail.inbox", "mail.message.list", ar.Const("folder", `"inbox"`))
	sent := mailProvider("mail.sent", "mail.message.list", ar.Const("folder", `"sent"`))
	deliveries := mailProvider("mail.deliveries", "mail.delivery.list")
	mailboxes := mailProvider("mail.mailboxes", "mail.mailbox.list")
	detail := mailProvider("mail.detail", "mail.message.get", ar.Bind("id", "route.id"))
	deliveries.Pagination, mailboxes.Pagination, detail.Pagination = "none", "none", "none"
	protocol := &ar.ApplicationV3{
		SchemaVersion: "3.0", ID: "mail", Version: "1.0.0", Source: "builtin:mail",
		RuntimeCompatibility: "3.0", LabelKey: "mail:heading", Permission: "mail:read", Route: "/mail",
		Pages: []ar.PageV3{
			mailPage("mail.inbox", "/mail", "mail:inbox", "mail:read", "inbox", inbox.ID, "inbox"),
			mailPage("mail.sent-page", "/mail/sent", "mail:sent", "mail:read", "sent", sent.ID, "sent"),
			mailPage("mail.compose-page", "/mail/compose", "mail:compose", "mail:send", "mailboxes", mailboxes.ID, "compose"),
			mailPage("mail.delivery-page", "/mail/deliveries", "mail:deliveries", "mail:read", "deliveries", deliveries.ID, "deliveries"),
			mailPage("mail.mailbox-page", "/mail/mailboxes", "mail:mailboxes", "mail:configure", "mailboxes", mailboxes.ID, "mailboxes"),
			mailPage("mail.detail-page", "/mail/:id", "mail:message", "mail:read", "message", detail.ID, "detail"),
		},
		Providers:  []ar.Provider{inbox, sent, detail, deliveries, mailboxes},
		Navigation: []ar.Navigation{{PageID: "mail.inbox", LabelKey: "mail:heading", Icon: "mail"}},
	}
	return ar.ApplicationV2{
		Protocol: protocol, SchemaVersion: "3.0", ID: protocol.ID, Version: protocol.Version,
		PluginVersion: "1.0.0", MetadataVersion: "1", Source: protocol.Source,
		RuntimeCompatibility: "3.0", LabelKey: protocol.LabelKey, Permission: protocol.Permission, Route: protocol.Route,
	}
}
