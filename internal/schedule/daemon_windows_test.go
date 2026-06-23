//go:build windows

package schedule

import "testing"

func TestDetachAttrCreationFlags(t *testing.T) {
	attr := detachAttr()
	if attr == nil {
		t.Fatal("detachAttr() = nil")
	}
	want := uint32(detachedProcess | createNewProcessGroup)
	if attr.CreationFlags != want {
		t.Errorf("CreationFlags = %#x, want %#x", attr.CreationFlags, want)
	}
}
