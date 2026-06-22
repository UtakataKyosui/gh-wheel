package notify

import (
	"runtime"
	"testing"
)

func TestQuote(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "hello", `"hello"`},
		{"double quote", `say "hi"`, `"say \"hi\""`},
		{"backslash", `a\b`, `"a\\b"`},
		{"both", `a"\b`, `"a\"\\b"`},
		{"empty", "", `""`},
		{"emoji and slash", "👀 3 / 📤 1", `"👀 3 / 📤 1"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := quote(tc.in); got != tc.want {
				t.Errorf("quote(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNotifyNoopOffDarwin(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("on darwin Notify shells out to osascript; covered manually")
	}
	if err := Notify("title", "message"); err != nil {
		t.Errorf("Notify on %s should be a no-op, got %v", runtime.GOOS, err)
	}
}

func TestAvailableReturnsForPlatform(t *testing.T) {
	got := Available()
	if runtime.GOOS != "darwin" && got {
		t.Errorf("Available() = true on %s, want false", runtime.GOOS)
	}
}
