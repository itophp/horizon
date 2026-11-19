package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

type project struct {
	root  string
	env   *localEnvironment
	tasks map[string]projectTask
}

type projectTask struct {
	run        func(project, []string) error
	acceptArgs bool
}

var projectTasks = map[string]projectTask{
	"install":   fixed(rootTool("pnpm", "install", "--frozen-lockfile")),
	"tools":     fixed(project.installTools),
	"build-cli": fixed(project.buildCLI),
	"bootstrap": fixed(goTool("run", "./cmd/bootstrap")),
	"dev":       fixed(rootTool("pnpm", "dev")),
	"dev-api":   fixed(goTool("run", "../cmd/api")),
	"db-up":     fixed(rootTool("docker", "compose", "up", "-d", "--wait")),
	"db-down":   fixed(rootTool("docker", "compose", "stop")),
	"migrate":   fixed(goTool("run", "./cmd/migrate")),
	"generate": fixed(
		goTool("run", "../cmd/workflowspec", "-out", "../docs/generated/workflows"),
		goTool("run", "goa.design/goa/v3/cmd/goa", "gen", "horizon/foundation/design"),
		project.generateSQL,
		rootTool("pnpm", "--filter", "@baseline/web", "generate"),
		goTool("run", "../cmd/runtimespec"),
		goTool("run", "../cmd/capabilitysdk"),
		rootTool("pnpm", "--filter", "@baseline/web", "exec", "prettier", "--write", "src/api/generated"),
	),
	"generate-workflows": fixed(goTool("run", "../cmd/workflowspec", "-out", "../docs/generated/workflows")),
	"check-workflows": fixed(
		goTool("test", "./kernel/workflow", "../cmd/workflowspec", "../cmd/workflowcheck"),
		goTool("run", "../cmd/workflowcheck", "-file", "../docs/generated/workflows/example-lifecycle.workflow.json"),
	),
	"check-generated":            fixed(project.checkGenerated),
	"check-entities":             fixed(goTool("run", "../cmd/entitycheck")),
	"test-orchestration-example": fixed(goTool("test", "-tags=integration", "-count=1", "-run", "TestExampleLifecycle", "-v", "../tests/integration")),
	"format":                     fixed(rootTool("pnpm", "format"), project.formatGo),
	"format-check":               fixed(rootTool("pnpm", "format:check"), project.checkGoFormat),
	"lint":                       fixed(rootTool("pnpm", "lint"), goTool("vet", "./...", "../applications/...", "../cmd/...", "../deployment/...")),
	"typecheck":                  fixed(rootTool("pnpm", "typecheck")),
	"test": fixed(
		rootTool("pnpm", "test"),
		goTool("test", "./...", "../applications/...", "../cmd/...", "../deployment/..."),
		rootTool("pnpm", "i18n:check"),
	),
	"build":             fixed(rootTool("pnpm", "build"), goTool("build", "./...", "../applications/...", "../cmd/...", "../deployment/...")),
	"test-integration":  fixed(goTool("test", "-tags=integration", "-count=1", "-v", "../tests/integration/...")),
	"test-e2e":          {run: project.testE2E, acceptArgs: true},
	"test-e2e-changed":  {run: project.testChangedE2E, acceptArgs: true},
	"test-e2e-smoke":    fixed(rootTool("pnpm", "test:e2e", "--grep", "@smoke")),
	"test-e2e-parallel": fixed(rootTool("bash", "scripts/e2e-parallel.sh")),
	"test-webmcp-e2e":   fixed(rootTool("pnpm", "test:e2e", "webmcp.spec.ts")),
	"check-rules":       fixed(project.checkRules),
	"ci-fast":           fixed(project.fastCI),
	"test-security":     fixed(project.testSecurity),
	"ci":                {run: project.ci, acceptArgs: true},
	"test-mcp": fixed(
		rootTool("pnpm", "mcp:test"),
		goTool("test", "./integrations/mcp"),
		goTool("test", "-tags=integration", "-count=1", "-run", "MCP", "../tests/integration/..."),
	),
	"test-mcp-e2e":        fixed(rootTool("pnpm", "test:e2e", "e2e/mcp.spec.ts$")),
	"test-mcp-native-e2e": fixed(project.testNativeMCPE2E),
	"test-mcp-native":     fixed(project.testNativeMCP),
}

func projectCommandNames() []string {
	names := make([]string, 0, len(projectTasks)+1)
	names = append(names, "init")
	for name := range projectTasks {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

func runProjectTask(task projectTask, name string, args []string) error {
	if slices.Equal(args, []string{"--help"}) || slices.Equal(args, []string{"-h"}) {
		suffix := ""
		switch name {
		case "test-e2e", "ci":
			suffix = " [Playwright arguments...]"
		case "test-e2e-changed":
			suffix = " [--dry-run]"
		}
		fmt.Printf("Usage: horizon-cli %s%s\n", name, suffix)
		return nil
	}
	if !task.acceptArgs && len(args) != 0 {
		return fmt.Errorf("usage: horizon-cli %s", name)
	}
	if name == "test-e2e-changed" && len(args) != 0 && !slices.Equal(args, []string{"--dry-run"}) {
		return fmt.Errorf("usage: horizon-cli test-e2e-changed [--dry-run]")
	}
	root, err := repositoryRoot()
	if err != nil {
		return err
	}
	env, err := loadEnvironment(root)
	if err != nil {
		return err
	}
	return task.run(project{root: root, env: env, tasks: projectTasks}, args)
}

func fixed(steps ...func(project) error) projectTask {
	return projectTask{run: func(p project, _ []string) error {
		for _, step := range steps {
			if err := step(p); err != nil {
				return err
			}
		}
		return nil
	}}
}

func rootTool(name string, args ...string) func(project) error {
	return func(p project) error { return runChild(p.command("", name, args...)) }
}

func goTool(args ...string) func(project) error {
	return func(p project) error { return runChild(p.goCommand(args...)) }
}

func (p project) command(dir, name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Dir = filepath.Join(p.root, dir)
	cmd.Env = p.env.childEnv()
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd
}

func (p project) goCommand(args ...string) *exec.Cmd {
	cmd := p.command("foundation", p.goExecutable("go"), args...)
	for _, setting := range []struct{ key, dir string }{{"GOMODCACHE", "gomod"}, {"GOCACHE", "gocache"}} {
		if p.env.get(setting.key) == "" {
			cmd.Env = append(cmd.Env, setting.key+"="+filepath.Join(p.root, ".tools", setting.dir))
		}
	}
	return cmd
}

func (p project) goExecutable(name string) string {
	local := filepath.Join(p.root, ".tools", "go", "bin", executableName(name))
	if info, err := os.Stat(local); err == nil && !info.IsDir() {
		return local
	}
	return name
}

func executableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

func runChild(cmd *exec.Cmd) error {
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(cmd.Path), err)
	}
	return nil
}

func (p project) installTools() error {
	cmd := p.goCommand("install", "github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1")
	cmd.Env = append(cmd.Env, "GOBIN="+filepath.Join(p.root, ".tools", "bin"))
	return runChild(cmd)
}

func (p project) buildCLI() error {
	return runChild(p.goCommand("build", "-trimpath", "-o", filepath.Join(p.root, executableName("horizon-cli")), "../cmd/horizon-cli"))
}

func (p project) generateSQL() error {
	sqlc := filepath.Join(p.root, ".tools", "bin", executableName("sqlc"))
	return runChild(p.command("foundation", sqlc, "generate"))
}

func (p project) cliChildEnv(cmd *exec.Cmd) error {
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	cmd.Env = append(cmd.Env, "HORIZON_CLI_EXECUTABLE="+binary)
	return nil
}

func (p project) checkGenerated() error {
	cmd := p.command("", "node", "scripts/check-generated.mjs")
	if err := p.cliChildEnv(cmd); err != nil {
		return err
	}
	return runChild(cmd)
}

func (p project) checkRules() error {
	cmd := p.command("", "pnpm", "rules:check")
	if err := p.cliChildEnv(cmd); err != nil {
		return err
	}
	return runChild(cmd)
}

func (p project) formatGo() error {
	return runChild(p.command("", p.goExecutable("gofmt"), "-w", "foundation", "applications", "cmd", "deployment", "tests", "internal"))
}

func (p project) checkGoFormat() error {
	cmd := p.command("", p.goExecutable("gofmt"), "-l", "foundation", "applications", "cmd", "deployment", "tests", "internal")
	cmd.Stdout = nil
	files, err := cmd.Output()
	fmt.Print(string(files))
	if err != nil {
		return fmt.Errorf("gofmt: %w", err)
	}
	if len(files) != 0 {
		return fmt.Errorf("Go files need formatting")
	}
	return nil
}

func (p project) testE2E(args []string) error {
	if err := p.checkE2EArgs(); err != nil {
		return err
	}
	return runChild(p.command("", "pnpm", append([]string{"test:e2e"}, args...)...))
}

func (p project) checkE2EArgs() error {
	if strings.TrimSpace(p.env.get("E2E_ARGS")) != "" {
		return fmt.Errorf("pass Playwright arguments directly on the command line and unset E2E_ARGS")
	}
	return nil
}

func (p project) testChangedE2E(args []string) error {
	return runChild(p.command("", "node", append([]string{"scripts/e2e-changed.mjs"}, args...)...))
}

func (p project) fastCI() error {
	return p.runTasks("check-rules", "format-check", "lint", "typecheck", "test", "check-generated", "check-entities", "check-workflows", "build")
}

func (p project) testSecurity() error {
	err := goTool("test", "./runtime/config", "./system/identity", "./kernel/capability/...", "./kernel/policy", "./integrations/mcp", "./runtime/transport")(p)
	if err != nil {
		return err
	}
	return p.runTasks("test-integration")
}

func (p project) ci(args []string) error {
	if err := p.checkE2EArgs(); err != nil {
		return err
	}
	if err := p.runTasks("ci-fast", "test-integration", "test-webmcp-e2e"); err != nil {
		return err
	}
	return p.testE2E(args)
}

func (p project) runTasks(names ...string) error {
	for _, name := range names {
		if err := p.tasks[name].run(p, nil); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

func (p project) testNativeMCPE2E() error {
	cmd := p.command("", "pnpm", "test:e2e", "e2e/mcp.spec.ts$")
	cmd.Env = append(cmd.Env, "MCP_NATIVE_ACCEPTANCE=1")
	return runChild(cmd)
}

func (p project) testNativeMCP() error {
	cmd := p.goCommand("test", "-tags=integration", "-count=1", "-timeout=3m", "-run", "TestMCPNativeClients", "-v", "../tests/integration/...")
	cmd.Env = append(cmd.Env, "MCP_NATIVE_ACCEPTANCE=1")
	return runChild(cmd)
}
