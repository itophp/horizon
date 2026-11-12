package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

func processBirth(pid int) (string, error) {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	// Field 2 is the parenthesized command and can contain spaces or parentheses.
	stat := string(raw)
	end := strings.LastIndex(stat, ")")
	fields := strings.Fields(stat[end+1:])
	if end < 0 || len(fields) <= 19 {
		return "", fmt.Errorf("invalid process stat for PID %d", pid)
	}
	if fields[0] == "Z" {
		return "", nil
	}
	return fields[19], nil // Field 22, start time in clock ticks since boot.
}
