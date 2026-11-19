package mail

import (
	"horizon/foundation/kernel/applicationruntime"
)

// RegisterUIContract contributes the trusted mail workspace component. The
// component renders sanitized HTML and invokes only mail capabilities through
// the generated Gateway client; the frontend implementation stays in apps/web.
func RegisterUIContract() {
	applicationruntime.RegisterUIContracts(applicationruntime.UIContract{
		Name:        "mail-workspace",
		Version:     "1.0",
		Category:    "element",
		Description: "Trusted personal-mail workspace. The component renders sanitized HTML and invokes only mail capabilities through the generated Gateway client.",
		PropsSchema: `{"type":"object","properties":{"mode":{"type":"string","enum":["inbox","sent","detail","compose","deliveries","mailboxes"]},"titleKey":{"type":"string"}},"required":["mode","titleKey"]}`,
		Events:      []string{},
		Slots:       []string{},
		Binding:     "data",
		ReadOnly:    "Preserve presentation; Gateway rechecks every provider and action.",
		Example:     `{"type":"element","id":"mail-workspace","element":{"kind":"mail-workspace","version":"1.0"}}`,
	})
}
