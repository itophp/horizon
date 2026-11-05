package app

import runtime "baseline/services/api/kernel/workflow/spec"

// WorkflowDefinitions is shared by the deployed composition root and offline generators.
func WorkflowDefinitions() []runtime.Definition {
	return append(runtime.Builtins(), runtime.ApplicationAnalyses()...)
}
