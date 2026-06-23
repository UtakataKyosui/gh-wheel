//go:build !windows

package schedule

import (
	"os/exec"
	"testing"
)

func TestDetachAttrSetsid(t *testing.T) {
	attr := detachAttr()
	if attr == nil || !attr.Setsid {
		t.Fatalf("detachAttr() = %+v, want Setsid: true", attr)
	}
}

func TestTerminateSignalsProcess(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot spawn child process: %v", err)
	}
	pid := cmd.Process.Pid
	if !alive(pid) {
		t.Fatalf("child %d should be alive right after Start", pid)
	}
	if err := terminate(pid); err != nil {
		t.Fatalf("terminate(%d): %v", pid, err)
	}
	// Wait reaps the child; it returns a non-nil error because the process was
	// killed by a signal rather than exiting cleanly.
	if err := cmd.Wait(); err == nil {
		t.Error("child exited cleanly; expected termination by SIGTERM")
	}
}
