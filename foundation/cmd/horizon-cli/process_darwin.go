package main

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

func processBirth(pid int) (string, error) {
	info, err := unix.SysctlKinfoProc("kern.proc.pid", pid)
	if errors.Is(err, unix.ESRCH) || errors.Is(err, unix.EIO) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	const zombie = 5 // SZOMB in Darwin's sys/proc.h
	p := info.Proc
	if int(p.P_pid) != pid || p.P_stat == zombie {
		return "", nil
	}
	return fmt.Sprintf("%d.%06d", p.P_starttime.Sec, p.P_starttime.Usec), nil
}
