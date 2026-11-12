//go:build !darwin && !linux && !windows

package main

import (
	"fmt"
	"os/exec"
)

var errPlatform = fmt.Errorf("service management is available on Windows, macOS and Linux")

func replaceFile(string, string) error { return errPlatform }

func lockFile(string) (func(), error)                { return nil, errPlatform }
func backgroundCommand(*exec.Cmd) (*exec.Cmd, error) { return nil, errPlatform }
func processStamp(int) (string, error)               { return "", errPlatform }
func terminateProcess(processIdentity) error         { return errPlatform }
func supervise([]string) error                       { return errPlatform }
