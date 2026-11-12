package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
	"golang.org/x/term"
)

type localEnvironment struct {
	path   string
	lines  []string
	values map[string]string
}

func envAssignment(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}
	line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
	key, value, ok := strings.Cut(line, "=")
	return strings.TrimSpace(key), strings.TrimSpace(value), ok && strings.TrimSpace(key) != ""
}

func loadEnvironment(root string) (*localEnvironment, error) {
	e := &localEnvironment{path: filepath.Join(root, ".env"), values: map[string]string{}}
	raw, err := os.ReadFile(e.path)
	if errors.Is(err, os.ErrNotExist) {
		return e, nil
	}
	if err != nil {
		return nil, err
	}
	e.lines = strings.Split(strings.ReplaceAll(strings.TrimPrefix(string(raw), "\uFEFF"), "\r\n", "\n"), "\n")
	for i, line := range e.lines {
		key, value, ok := envAssignment(line)
		if !ok {
			if line = strings.TrimSpace(line); line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			return nil, fmt.Errorf(".env line %d: expected KEY=value", i+1)
		}
		switch {
		case strings.HasPrefix(value, "\""):
			value, err = strconv.Unquote(value)
			if err != nil {
				return nil, fmt.Errorf(".env line %d: invalid quoted value", i+1)
			}
		case strings.HasPrefix(value, "'"):
			if len(value) < 2 || !strings.HasSuffix(value, "'") {
				return nil, fmt.Errorf(".env line %d: invalid quoted value", i+1)
			}
			value = value[1 : len(value)-1]
		default:
			if cut := strings.IndexAny(value, " \t"); cut >= 0 && strings.HasPrefix(strings.TrimSpace(value[cut:]), "#") {
				value = value[:cut]
			}
		}
		e.values[key] = value
	}
	return e, nil
}

func (e *localEnvironment) get(key string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return e.values[key]
}

func (e *localEnvironment) childEnv() []string {
	env := []string{}
	for key, value := range e.values {
		if _, present := os.LookupEnv(key); !present {
			env = append(env, key+"="+value)
		}
	}
	return append(env, os.Environ()...)
}

func (e *localEnvironment) save(updates map[string]string) error {
	lines := slices.Clone(e.lines)
	for _, key := range []string{"DATABASE_URL", "REDIS_URL"} {
		value, ok := updates[key]
		if !ok {
			continue
		}
		// Save literal URLs so existing .env consumers also read these values.
		line := key + "=" + value
		found := false
		for i, old := range lines {
			k, _, assignment := envAssignment(old)
			if assignment && key == k {
				lines[i], found = line, true
			}
		}
		if !found {
			if len(lines) > 0 && lines[len(lines)-1] == "" {
				lines = lines[:len(lines)-1]
			}
			lines = append(lines, line, "")
		}
	}
	file, err := os.CreateTemp(filepath.Dir(e.path), ".env.horizon-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.WriteString(strings.Join(lines, "\n"))
	if err = errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	if err = replaceFile(file.Name(), e.path); err != nil {
		return err
	}
	e.lines = lines
	for key, value := range updates {
		e.values[key] = value
	}
	return nil
}

func checkURL(key, value string) error {
	prefix := []string{"postgres://", "postgresql://"}
	if key == "REDIS_URL" {
		prefix = []string{"redis://", "rediss://"}
	}
	valid := false
	for _, scheme := range prefix {
		valid = valid || strings.HasPrefix(value, scheme)
	}
	if !valid || strings.ContainsAny(value, " \t\r\n#") {
		return fmt.Errorf("%s: use a connection URL and percent-encode reserved characters", key)
	}
	if key == "REDIS_URL" {
		if _, err := redis.ParseURL(value); err != nil {
			return fmt.Errorf("REDIS_URL: invalid connection URL")
		}
	} else if _, err := pgx.ParseConfig(value); err != nil {
		return fmt.Errorf("DATABASE_URL: invalid connection URL")
	}
	return nil
}

func (e *localEnvironment) configure() error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("configuration needs an interactive terminal; set DATABASE_URL and optional REDIS_URL in .env or in the process environment")
	}
	updates := map[string]string{}
	for _, key := range []string{"DATABASE_URL", "REDIS_URL"} {
		hint := "required, input hidden"
		if e.get(key) != "" {
			hint = "Enter to keep existing value, input hidden"
		} else if key == "REDIS_URL" {
			hint = "optional, Enter to skip, input hidden"
		}
		for {
			fmt.Fprintf(os.Stderr, "%s (%s): ", key, hint)
			input, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return err
			}
			value := strings.TrimSpace(string(input))
			if value == "" && (e.get(key) != "" || key == "REDIS_URL") {
				break
			}
			if err = checkURL(key, value); err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}
			if _, overridden := os.LookupEnv(key); overridden {
				fmt.Fprintf(os.Stderr, "The process environment still takes precedence for %s.\n", key)
			}
			updates[key] = value
			break
		}
	}
	if len(updates) == 0 {
		return nil
	}
	return e.save(updates)
}

type envCheck struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Required bool   `json:"required"`
	Hint     string `json:"hint,omitempty"`
}

func environmentCommand(args []string) error {
	if len(args) == 0 || args[0] != "check" && args[0] != "configure" {
		return fmt.Errorf("expected env check [all|frontend|backend] or env configure")
	}
	if args[0] == "configure" && len(args) != 1 {
		return fmt.Errorf("env configure takes no arguments")
	}
	selected, err := selectServices(args[1:])
	if err != nil {
		return err
	}
	root, err := repositoryRoot()
	if err != nil {
		return err
	}
	if args[0] == "configure" {
		unlock, err := developmentLock(root)
		if err != nil {
			return err
		}
		defer unlock()
		e, err := loadEnvironment(root)
		if err != nil {
			return err
		}
		if err = e.configure(); err != nil {
			return err
		}
		return printJSON(map[string]string{"configFile": e.path, "database": configured(e.get("DATABASE_URL")), "redis": configured(e.get("REDIS_URL"))})
	}
	e, err := loadEnvironment(root)
	if err != nil {
		return err
	}
	_, checks := e.inspect(root, selected)
	return errors.Join(printJSON(checks), requiredChecks(checks))
}

func configured(value string) string {
	if value == "" {
		return "unconfigured"
	}
	return "configured"
}

func requiredChecks(checks []envCheck) error {
	var problems []error
	for _, check := range checks {
		if check.Required && check.Status != "ok" {
			problems = append(problems, fmt.Errorf("%s: %s", check.Name, check.Hint))
		}
	}
	return errors.Join(problems...)
}

func (e *localEnvironment) inspect(root string, selected []string) (map[string]*exec.Cmd, []envCheck) {
	commands := map[string]*exec.Cmd{}
	checks := []envCheck{}
	record := func(name string, ok, required bool, hint string) {
		status := "ok"
		if !ok {
			status = "unavailable"
		} else {
			hint = ""
		}
		checks = append(checks, envCheck{name, status, required, hint})
	}
	addr := e.get("API_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}
	if slices.Contains(selected, "backend") {
		goPath := filepath.Join(root, ".tools", "go", "bin", "go")
		if runtime.GOOS == "windows" {
			goPath += ".exe"
		}
		if _, err := os.Stat(goPath); err != nil {
			goPath, _ = exec.LookPath("go")
		}
		record("go", goPath != "", true, "install the Go toolchain from go.mod")
		host, port, err := net.SplitHostPort(addr)
		portNumber, portErr := strconv.Atoi(port)
		record("api_address", err == nil && portErr == nil && portNumber > 0 && portNumber <= 65535 && net.ParseIP(host) != nil, true, "set API_ADDR to an IP address and port, e.g. 127.0.0.1:8080")
		checks = append(checks, probeDatabase(e.get("DATABASE_URL")), probeRedis(e.get("REDIS_URL")))
		if goPath != "" {
			cmd := exec.Command(goPath, "run", "./cmd/api")
			cmd.Dir = filepath.Join(root, "foundation")
			cmd.Env = e.childEnv()
			for _, setting := range []struct{ key, dir string }{{"GOMODCACHE", "gomod"}, {"GOCACHE", "gocache"}} {
				if e.get(setting.key) == "" {
					cmd.Env = append(cmd.Env, setting.key+"="+filepath.Join(root, ".tools", setting.dir))
				}
			}
			commands["backend"] = cmd
		}
	}
	if slices.Contains(selected, "frontend") {
		node, err := exec.LookPath("node")
		record("node", err == nil, true, "install Node.js as specified in apps/web/package.json")
		web := filepath.Join(root, "apps", "web")
		vite := filepath.Join(web, "node_modules", "vite", "bin", "vite.js")
		_, viteErr := os.Stat(vite)
		record("web_dependencies", viteErr == nil, true, "run pnpm install --frozen-lockfile in the checkout")
		port := e.get("WEB_PORT")
		if port == "" {
			port = "5173"
		}
		p, err := strconv.Atoi(port)
		record("web_port", err == nil && p > 0 && p <= 65535, true, "set WEB_PORT to a TCP port")
		if node != "" && viteErr == nil {
			cmd := exec.Command(node, vite, "--host", "127.0.0.1", "--port", port, "--strictPort")
			cmd.Dir, cmd.Env = web, e.childEnv()
			if e.get("API_PROXY_TARGET") == "" {
				cmd.Env = append(cmd.Env, "API_PROXY_TARGET=http://"+addr)
			}
			commands["frontend"] = cmd
		}
	}
	return commands, checks
}

func probeDatabase(value string) envCheck {
	check := envCheck{"postgresql", "unconfigured", true, "run horizon-cli env configure or set DATABASE_URL"}
	if value == "" {
		return check
	}
	check.Status = "unavailable"
	if checkURL("DATABASE_URL", value) != nil {
		check.Hint = "DATABASE_URL is invalid; run horizon-cli env configure"
		return check
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, value)
	if err != nil {
		check.Hint = "connection failed; verify PostgreSQL is running and check DATABASE_URL"
		return check
	}
	conn.Close(ctx)
	check.Status, check.Hint = "ok", ""
	return check
}

func probeRedis(value string) envCheck {
	check := envCheck{"redis", "unconfigured", false, "set REDIS_URL for an explicit Redis check and knowledge vector store override"}
	if value == "" {
		return check
	}
	check.Status = "unavailable"
	options, err := redis.ParseURL(value)
	if err != nil || checkURL("REDIS_URL", value) != nil {
		check.Hint = "REDIS_URL is invalid; run horizon-cli env configure"
		return check
	}
	options.MaxRetries = -1
	client := redis.NewClient(options)
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if err = client.Ping(ctx).Err(); err != nil {
		check.Hint = "connection failed; verify the Redis endpoint and credentials"
		return check
	}
	check.Status, check.Hint = "ok", ""
	return check
}

func prepareServices(root string, selected []string) (map[string]*exec.Cmd, error) {
	e, err := loadEnvironment(root)
	if err != nil {
		return nil, err
	}
	if slices.Contains(selected, "backend") && e.get("DATABASE_URL") == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL is required to start the backend.")
		if err = e.configure(); err != nil {
			return nil, err
		}
	}
	commands, checks := e.inspect(root, selected)
	for _, check := range checks {
		if !check.Required && check.Status == "unavailable" {
			fmt.Fprintf(os.Stderr, "%s: %s\n", check.Name, check.Hint)
		}
	}
	return commands, requiredChecks(checks)
}
