package app

import runtime "horizon/foundation/kernel/workflow/spec"

// WorkflowDefinitions is the platform workflow contribution shared by the
// deployed composition root and offline generators. Application workflows
// travel with their plugin packages and are assembled by the deployment.
func WorkflowDefinitions() []runtime.Definition {
	return []runtime.Definition{runtime.RuntimeDesigner()}
}

// GraphDefinitions is the platform graph contribution. Application graphs are
// contributed by their plugins and assembled by the deployment.
func GraphDefinitions() []runtime.GraphDefinition { return nil }
