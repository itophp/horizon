// Command horizon-cli inspects compiled development contracts and manages local dev processes.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
)

const usage = `horizon-cli
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

Catalog output is JSON. Service state and logs live in .local/horizon-cli/.`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "horizon-cli:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	args = slices.DeleteFunc(args, func(arg string) bool { return arg == "--json" })
	if len(args) == 0 || slices.Contains([]string{"help", "-h", "--help"}, args[0]) {
		fmt.Println(usage)
		return nil
	}
	switch args[0] {
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
