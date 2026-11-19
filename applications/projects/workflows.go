package projects

import (
	workflow "horizon/foundation/kernel/workflow/spec"
)

// Workflows are the projects application's durable flows. They travel with
// the application; the kernel workflow engine only executes definitions the
// deployment assembles from plugins.
func Workflows() []workflow.Definition {
	return []workflow.Definition{workflow.RuntimeAnalysis("project", "projects", "project.detail", "project.task.list@1.0.0")}
}
