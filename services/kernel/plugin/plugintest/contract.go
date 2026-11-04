// Package plugintest provides a reusable offline contract check for builtin plugins.
package plugintest

import (
	"context"
	"testing"

	"baseline/services/api/kernel/plugin"
)

// Contract verifies that a plugin can be registered without starting it and
// that every published extension is attributed to the plugin's manifest.
func Contract(t testing.TB, p plugin.Plugin) *plugin.Manager {
	t.Helper()
	manager := plugin.New(p)
	if err := manager.Register(context.Background()); err != nil {
		t.Fatalf("plugin contract: %v", err)
	}
	manifest := p.Manifest()
	for _, extension := range manager.Catalog.Extensions {
		if extension.PluginID != manifest.ID || extension.PluginVersion != manifest.Version || extension.Version == "" {
			t.Fatalf("invalid extension ownership: %+v", extension)
		}
	}
	return manager
}
