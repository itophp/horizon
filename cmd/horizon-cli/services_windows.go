package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func replaceFile(source, destination string) error {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}

func lockFile(path string) (func(), error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	var offset windows.Overlapped
	if err = windows.LockFileEx(windows.Handle(file.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &offset); err != nil {
		file.Close()
		return nil, err
	}
	return func() { file.Close() }, nil
}

func backgroundCommand(child *exec.Cmd) (*exec.Cmd, error) {
	binary, err := os.Executable()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(binary, append([]string{"_supervise", child.Path}, child.Args[1:]...)...)
	cmd.Dir, cmd.Env = child.Dir, child.Env
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP}
	return cmd, nil
}

// Joining the job before spawning the service makes every descendant inherit
// it. Windows closes the supervisor's job handle on exit and stops the tree.
func supervise(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing service command")
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	assigned := false
	defer func() {
		if !assigned {
			windows.CloseHandle(job)
		}
	}()
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return err
	}
	self, err := windows.GetCurrentProcess()
	if err != nil {
		return err
	}
	if err = windows.AssignProcessToJobObject(job, self); err != nil {
		return err
	}
	assigned = true
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}

func birth(handle windows.Handle) (string, error) {
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return "", err
	}
	state, err := windows.WaitForSingleObject(handle, 0)
	if err != nil || state == windows.WAIT_OBJECT_0 {
		return "", err
	}
	return strconv.FormatInt(creation.Nanoseconds(), 10), nil
}

func processStamp(pid int) (string, error) {
	if pid <= 1 {
		return "", nil
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(pid))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)
	return birth(handle)
}

func terminateProcess(p processIdentity) error {
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE, false, uint32(p.PID))
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return nil
	}
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	stamp, err := birth(handle)
	if err != nil || stamp != p.Stamp {
		return err
	}
	if err = windows.TerminateProcess(handle, 0); err != nil {
		return err
	}
	state, err := windows.WaitForSingleObject(handle, 8000)
	if err == nil && state != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("service supervisor %d has not stopped", p.PID)
	}
	return err
}
