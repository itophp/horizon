// Command horizon-cli inspects contracts, manages local services, and runs project tasks.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

const usage = `horizon-cli
  init [--repo <url-or-path>] [directory]
  capability|workflow|graph|application|page [--full] [--json]
  catalog [--full] [--json]
  search <query> [--json]
  capability schema [name[@version]]
  workflow schema [name[@version]]
  graph schema [name[@version]]
  application schema [id[@version]]
  page schema [app/page[@version]]
  capability|workflow|graph|application|page get <id> [--json]
  refs capability:<name[@version]> [--json]
  validate workflow <file> [--json]
  env check [all|frontend|backend]
  env configure
  start|stop|restart|status [all|frontend|backend] [--json]
  commands [--json]
  test-e2e|ci [Playwright arguments...]
  test-e2e-changed [--dry-run]

Catalog output is JSON. Service state and logs live in .local/horizon-cli/.`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "horizon-cli:", err)
		var childExit *exec.ExitError
		if errors.As(err, &childExit) && childExit.ExitCode() > 0 {
			os.Exit(childExit.ExitCode())
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 0 {
		if args[0] == "init" {
			return initProject(args[1:])
		}
		if task, ok := projectTasks[args[0]]; ok {
			return runProjectTask(task, args[0], args[1:])
		}
	}
	args = slices.DeleteFunc(args, func(arg string) bool { return arg == "--json" })
	if len(args) == 0 || slices.Contains([]string{"help", "-h", "--help"}, args[0]) {
		fmt.Println(usage)
		fmt.Println("\nProject commands:", strings.Join(projectCommandNames(), ", "))
		return nil
	}
	switch args[0] {
	case "commands":
		if len(args) != 1 {
			return fmt.Errorf("usage: horizon-cli commands [--json]")
		}
		return printJSON(projectCommandNames())
	case "_supervise":
		return supervise(args[1:])
	case "env":
		return environmentCommand(args[1:])
	case "start", "stop", "restart", "status":
		return services(args[0], args[1:])
	default:
		return inspect(args)
	}
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
