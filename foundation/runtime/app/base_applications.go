package app

import (
	ar "horizon/foundation/kernel/applicationruntime"
)

// baseApplicationPlugins enumerates platform-managed application packages.
// They are part of the product itself: every tenant always has them enabled,
// their definitions follow the deployment baseline, and platform releases
// upgrade them automatically. They are not installable tenant packages.
// Tenant-designed applications (runtime-host) and business modules stay
// installable and upgradable through the application management surface.
var baseApplicationPlugins = map[string]bool{
	"builtin:automation":           true, // AI workbench scheduling pages
	"system.identity":              true, // accounts, organizations, permissions, runtime-protocol
	"builtin:system-settings":      true, // system settings and AI configuration
	"builtin:capabilities":         true,
	"builtin:analytics":            true,
	"builtin:agent-collaboration":  true,
	"builtin:personal-preferences": true,
}

// BaseApplicationPlugins copies the platform-managed package classification for
// runtime composition and offline spec generation.
func BaseApplicationPlugins() map[string]bool {
	out := make(map[string]bool, len(baseApplicationPlugins))
	for id := range baseApplicationPlugins {
		out[id] = true
	}
	return out
}

// ClassifyCandidates applies the base classification to deployment candidates.
func ClassifyCandidates(candidates []ar.Candidate) []ar.Candidate {
	return ar.MarkBase(candidates, baseApplicationPlugins)
}
