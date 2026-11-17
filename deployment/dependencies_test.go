package deployment

import (
	"os/exec"
	"strings"
	"testing"
)

func TestFoundationDoesNotDependOnApplications(t *testing.T) {
	command := exec.Command("go", "list", "-deps", "-test", "./foundation/...")
	command.Dir = ".."
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("list foundation dependencies: %v\n%s", err, output)
	}
	for _, dependency := range strings.Fields(string(output)) {
		if strings.HasPrefix(dependency, "horizon/applications/") || dependency == "horizon/deployment" {
			t.Errorf("foundation depends on %s", dependency)
		}
	}
}
