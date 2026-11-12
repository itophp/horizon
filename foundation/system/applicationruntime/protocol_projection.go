package applicationruntime

import (
	"encoding/json"
	"slices"
	"strings"

	ar "horizon/foundation/kernel/applicationruntime"
	"horizon/foundation/kernel/capability/contracts"
	"horizon/foundation/system/identity"
)

func canOpenRuntime(principal *identity.Principal, permission string) bool {
	return permission == ar.AuthenticatedPermission && principal.Account.ID > 0 ||
		principal.Can(permission)
}

// projectProtocol removes pages whose page or provider permission is not
// granted. The frontend is only a presentation gate; every provider and event
// still passes through the Gateway with the current principal.
func (m *Module) projectProtocol(source ar.ApplicationV3, principal *identity.Principal) *ar.ApplicationV3 {
	protocol := ar.Canonical(source)
	providers := []ar.Provider{}
	allowedProviders := map[string]bool{}
	for _, provider := range protocol.Providers {
		if (provider.Permission == "" && ar.PublicSystemProvider(provider.Adapter)) || canOpenRuntime(principal, provider.Permission) {
			providers = append(providers, provider)
			allowedProviders[provider.ID] = true
		}
	}
	protocol.Providers = providers

	pages := []ar.PageV3{}
	visible := map[string]bool{}
	for _, page := range protocol.Pages {
		visible[page.ID] = canOpenRuntime(principal, page.Permission) && protocolLayoutAccessible(page.Root, allowedProviders)
		if visible[page.ID] {
			pages = append(pages, page)
		}
	}
	protocol.Pages = pages
	events := m.allowedEventTargets(principal, source)
	for i := range protocol.Pages {
		projectProtocolNode(
			protocol.Pages[i].Root, visible, allowedProviders,
			events, principal, m,
		)
	}
	navigation := []ar.Navigation{}
	for _, item := range protocol.Navigation {
		if visible[item.PageID] {
			navigation = append(navigation, item)
		}
	}
	protocol.Navigation = navigation
	return &protocol
}

func (m *Module) allowedEventTargets(principal *identity.Principal, source ar.ApplicationV3) map[string]bool {
	authenticated := source.Permission == ar.AuthenticatedPermission &&
		principal.Account.ID > 0 && slices.ContainsFunc(m.contributions(), func(app ar.ApplicationV2) bool {
		return app.ID == source.ID && app.Source == source.Source && app.Permission == ar.AuthenticatedPermission
	})
	allowed := map[string]bool{}
	for key, definition := range m.Definitions {
		allowed[key] = principal.Can(definition.Permission) || (authenticated && isAuthenticatedAction(definition))
	}
	return allowed
}

func isAuthenticatedAction(definition contracts.Definition) bool {
	return !definition.ReadOnly && definition.Permission == "" &&
		slices.Contains(definition.Tags, contracts.AuthenticatedActionTag) &&
		!slices.Contains(definition.Tags, "internal-only") && !slices.Contains(definition.Tags, "system-only")
}

func protocolLayoutAccessible(node ar.Node, providers map[string]bool) bool {
	if node.Type != "layout" || node.Layout == nil {
		return false
	}
	for _, data := range node.Layout.Data {
		if data.Provider != "" && !strings.HasPrefix(data.ID, "_") && !providers[data.Provider] {
			return false
		}
	}
	for _, child := range node.Layout.Children {
		if child.Component != nil && child.Component.Content != nil &&
			!protocolLayoutAccessible(ar.Node{Type: "layout", Layout: child.Component.Content}, providers) {
			return false
		}
		if child.Type == "layout" && !protocolLayoutAccessible(child, providers) {
			return false
		}
	}
	return true
}

func projectProtocolNode(
	node ar.Node,
	pages, providers, events map[string]bool,
	principal *identity.Principal,
	module *Module,
) {
	if node.Type != "layout" || node.Layout == nil {
		return
	}
	for _, child := range node.Layout.Children {
		projectProtocolNode(child, pages, providers, events, principal, module)
		if child.Component == nil {
			continue
		}
		if content := child.Component.Content; content != nil {
			projectProtocolNode(ar.Node{Type: "layout", Layout: content}, pages, providers, events, principal, module)
		}
		module.projectProtocolFormProps(child.Component.Props, principal)
		bindings := []ar.EventBinding{}
		for _, event := range child.Component.Events {
			switch event.Action {
			case "navigate":
				if pages[event.Target] {
					bindings = append(bindings, event)
				}
			case "invoke":
				if name, version, ok := strings.Cut(event.Target, "@"); ok && events[name+"@"+version] {
					bindings = append(bindings, event)
				}
			case "setData", "refresh", "system":
				bindings = append(bindings, event)
			}
		}
		child.Component.Events = bindings
	}
}

func (m *Module) projectProtocolFormProps(
	props map[string]any,
	p *identity.Principal,
) {
	if value, ok := props["form"]; ok {
		if !m.projectProtocolForm(value, p) {
			delete(props, "form")
		}
	}
	if forms, ok := props["forms"].(map[string]any); ok {
		for key, value := range forms {
			if !m.projectProtocolForm(value, p) {
				delete(forms, key)
			}
		}
		if len(forms) == 0 {
			delete(props, "forms")
		}
	}
}

func (m *Module) projectProtocolForm(
	value any,
	p *identity.Principal,
) bool {
	raw, err := json.Marshal(value)
	if err != nil {
		return false
	}
	var form ar.FormV2
	if err := json.Unmarshal(raw, &form); err != nil {
		return false
	}
	if form.Create.Permission == "" {
		definition := m.Definitions[form.Create.Capability.Name+"@"+form.Create.Capability.Version]
		form.Create.Permission = definition.Permission
	}
	if form.Edit.Permission == "" {
		definition := m.Definitions[form.Edit.Capability.Name+"@"+form.Edit.Capability.Version]
		form.Edit.Permission = definition.Permission
	}
	if form.Create.Permission != "" && !p.Can(form.Create.Permission) {
		form.Create = ar.Command{}
	}
	if form.Edit.Permission != "" && !p.Can(form.Edit.Permission) {
		form.Edit = ar.Command{}
	}
	if form.Create.Capability.Name != "" {
		form.Create = m.projectCommand(form.Create, p)
	}
	if form.Edit.Capability.Name != "" {
		form.Edit = m.projectCommand(form.Edit, p)
	}
	if form.Create.Capability.Name == "" && form.Edit.Capability.Name == "" {
		return false
	}
	raw, err = json.Marshal(form)
	if err != nil {
		return false
	}
	var replacement map[string]any
	if err := json.Unmarshal(raw, &replacement); err != nil {
		return false
	}
	if target, ok := value.(map[string]any); ok {
		for key, item := range replacement {
			target[key] = item
		}
	}
	return true
}
