package app

import runtime "baseline/services/api/kernel/workflow/spec"

// WorkflowDefinitions is shared by the deployed composition root and offline generators.
func WorkflowDefinitions() []runtime.Definition {
	return append(append(runtime.Builtins(), runtime.ApplicationAnalyses()...), runtime.RuntimeDesigner())
}

// GraphDefinitions is the shared deployment catalog for runtime and offline checks.
func GraphDefinitions() []runtime.GraphDefinition { return runtime.BuiltinGraphs() }
