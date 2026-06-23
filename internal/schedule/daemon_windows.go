//go:build windows

package schedule

import (
	"os"
	"syscall"
)

// Windows process-creation flags (from the Win32 CreateProcess API) used to
// detach the daemon from the parent console so it keeps running after the CLI
// exits. Windows has no setsid; this is the closest equivalent.
const (
	detachedProcess       = 0x00000008 // DETACHED_PROCESS
	createNewProcessGroup = 0x00000200 // CREATE_NEW_PROCESS_GROUP
)

// detachAttr returns the SysProcAttr that detaches the daemon on Windows.
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: detachedProcess | createNewProcessGroup}
}

// alive reports whether a process with the given pid exists. On Windows
// os.FindProcess opens a real OS handle and returns an error when the process
// is gone, so a successful open means the process exists; we release the handle
// immediately to avoid leaking it. (syscall.Signal(0), used on Unix, is not
// supported on Windows.)
func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	_ = proc.Release()
	return true
}

// terminate stops the process. Windows has no SIGTERM, so Kill (TerminateProcess)
// is the portable option; the handle is released afterwards to avoid a leak.
func terminate(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	defer func() { _ = proc.Release() }()
	return proc.Kill()
}
