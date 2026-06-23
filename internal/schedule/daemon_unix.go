//go:build !windows

package schedule

import (
	"os"
	"syscall"
)

// detachAttr returns the SysProcAttr that detaches the daemon from the
// controlling terminal so it survives the parent CLI exiting. On Unix this is a
// new session (setsid); see daemon_windows.go for the Windows equivalent.
func detachAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

// alive reports whether a process with the given pid exists and is signalable.
// The null signal (0) runs the kernel's permission/existence check without
// actually delivering a signal. os.FindProcess never fails on Unix, so the
// liveness check has to be done with the signal.
func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// terminate asks the process to shut down gracefully with SIGTERM, letting the
// daemon run its deferred cleanup (removing its own pid file).
func terminate(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(syscall.SIGTERM)
}
