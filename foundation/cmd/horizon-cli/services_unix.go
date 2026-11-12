//go:build darwin || linux

package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func replaceFile(source, destination string) error { return os.Rename(source, destination) }

func lockFile(path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		file.Close()
		return nil, err
	}
	return func() { file.Close() }, nil
}

func backgroundCommand(cmd *exec.Cmd) (*exec.Cmd, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd, nil
}

func processStamp(pid int) (string, error) {
	if pid <= 1 {
		return "", nil
	}
	pgid, err := syscall.Getpgid(pid)
	if errors.Is(err, syscall.ESRCH) || err == nil && pgid != pid {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return processBirth(pid)
}

func terminateProcess(p processIdentity) error {
	stamp, err := processStamp(p.PID)
	if err != nil || stamp != p.Stamp {
		return err
	}
	if err = syscall.Kill(-p.PID, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("signal process group %d: %w", p.PID, err)
	}
	deadline := time.Now().Add(8 * time.Second)
	for {
		err = syscall.Kill(-p.PID, 0)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil && !errors.Is(err, syscall.EPERM) {
			return fmt.Errorf("check process group %d: %w", p.PID, err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("process group %d has not stopped after SIGTERM", p.PID)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func supervise([]string) error {
	return fmt.Errorf("the service supervisor is specific to Windows")
}
