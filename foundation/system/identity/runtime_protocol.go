package identity

import ar "horizon/foundation/kernel/applicationruntime"

// RuntimeProtocolApplication is the deployment-owned V3 smoke definition. It
// uses a real identity capability so catalog assembly, authorization projection
// and Gateway reads follow the same path as production pages.
func RuntimeProtocolApplication() ar.ApplicationV2 {
	provider := ar.Provider{
		ID:          "protocol.accounts",
		Capability:  ar.Ref("identity.accounts.admin.read"),
		ResultPath:  "items",
		KeyField:    "id",
		LabelField:  "fullName",
		Pagination:  "none",
		StaleTimeMs: 15000,
		Tags:        []string{"identity", "runtime-protocol"},
	}
	protocol := &ar.ApplicationV3{
		SchemaVersion:        "3.0",
		ID:                   "runtime-protocol",
		Version:              "2.3.0",
		Source:               "system.identity",
		RuntimeCompatibility: "3.0",
		LabelKey:             "runtime:protocol.heading",
		Permission:           "accounts:read",
		Route:                "/runtime-protocol",
		Pages: []ar.PageV3{{
			SchemaVersion: "3.0",
			ID:            "runtime-protocol.accounts",
			Route:         "/runtime-protocol",
			TitleKey:      "runtime:protocol.heading",
			SubtitleKey:   "runtime:protocol.subtitle",
			Permission:    "accounts:read",
			Root: ar.Node{
				Type: "layout",
				Layout: &ar.LayoutSpec{
					ID:   "page",
					Kind: "stack",
					Props: ar.LayoutProps{
						Gap:   "20px",
						Width: "full",
					},
					Data: []ar.DataSpec{{ID: "accounts", Provider: provider.ID}},
					Children: []ar.Node{
						{
							Type: "layout",
							Layout: &ar.LayoutSpec{
								ID:   "heading",
								Kind: "grid",
								Props: ar.LayoutProps{
									Columns: "minmax(0, 1fr)",
								},
								Children: []ar.Node{
									{
										Type: "component",
										Component: &ar.ComponentSpec{
											ID:      "heading",
											Kind:    "page-heading",
											Version: "1.0",
											Props: map[string]any{
												"titleKey":    "runtime:protocol.heading",
												"subtitleKey": "runtime:protocol.subtitle",
											},
										},
									},
								},
							},
						},
						{
							Type: "layout",
							Layout: &ar.LayoutSpec{
								ID:    "accounts-panel",
								Kind:  "grid",
								Props: ar.LayoutProps{Columns: "minmax(0, 1fr)"},
								Children: []ar.Node{{
									Type: "component",
									Component: &ar.ComponentSpec{
										ID: "accounts-card", Kind: "card", Version: "1.0",
										Props: map[string]any{},
										Content: &ar.LayoutSpec{
											ID: "accounts-content", Kind: "grid",
											Children: []ar.Node{
												{
													Type: "component",
													Component: &ar.ComponentSpec{
														ID:      "accounts",
														Kind:    "data-table",
														Version: "1.0",
														Data:    "$accounts",
														Props: map[string]any{
															"keyField": "id",
															"columns": []map[string]any{
																{"name": "fullName", "labelKey": "admin:fields.fullName", "display": "entity"},
																{"name": "username", "labelKey": "admin:fields.username", "display": "mono"},
																{"name": "email", "labelKey": "admin:fields.email"},
																{"name": "status", "labelKey": "admin:fields.status", "display": "status"},
																{"name": "createdAt", "labelKey": "admin:fields.createdAt", "display": "date"},
															},
														},
													},
												},
											},
										},
									},
								}},
							},
						},
					},
				},
			},
		}},
		Providers:  []ar.Provider{provider},
		Navigation: []ar.Navigation{{PageID: "runtime-protocol.accounts", LabelKey: "runtime:protocol.nav", Icon: "boxes"}},
	}
	return ar.ApplicationV2{
		Protocol:             protocol,
		SchemaVersion:        "3.0",
		ID:                   protocol.ID,
		Version:              protocol.Version,
		PluginVersion:        "2.3.0",
		MetadataVersion:      "1",
		Source:               protocol.Source,
		RuntimeCompatibility: "3.0",
		LabelKey:             protocol.LabelKey,
		Permission:           protocol.Permission,
		Route:                protocol.Route,
	}
}
