package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const defaultProjectRepository = "https://github.com/itophp/horizon.git"
const initUsage = "Usage: horizon-cli init [--repo <url-or-path>] [directory]"

func initProject(args []string) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	repository := flags.String("repo", defaultProjectRepository, "source Git repository")
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), initUsage)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() > 1 || strings.TrimSpace(*repository) == "" {
		return fmt.Errorf("%s", initUsage)
	}
	directory := "horizon"
	if flags.NArg() == 1 {
		directory = flags.Arg(0)
		if strings.TrimSpace(directory) == "" {
			return fmt.Errorf("project directory must not be empty")
		}
	}

	target, err := filepath.Abs(directory)
	if err != nil {
		return err
	}
	clone := exec.Command("git", "clone", "--", *repository, target)
	if clone.Err != nil {
		return fmt.Errorf("Git is required to initialize a project: %w", clone.Err)
	}
	if err = os.Mkdir(target, 0o755); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("project directory already exists: %s", target)
		}
		return fmt.Errorf("create project directory: %w", err)
	}
	clone.Stdin, clone.Stdout, clone.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err = runChild(clone); err != nil {
		// Git cleans up failed clones. Remove the directory only if it is empty.
		if cleanupErr := os.Remove(target); cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
			return errors.Join(err, fmt.Errorf("remove incomplete project directory: %w", cleanupErr))
		}
		return err
	}
	fmt.Printf("Initialized Horizon project in %s\n", target)
	return nil
}
