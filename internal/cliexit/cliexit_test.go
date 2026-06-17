package cliexit_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/UtakataKyosui/gh-wheel/internal/cliexit"
)

// ─── ExitCodeOf ──────────────────────────────────────────────────────────────

func TestExitCodeOf_Nil(t *testing.T) {
	if got := cliexit.ExitCodeOf(nil); got != cliexit.CodeSuccess {
		t.Errorf("ExitCodeOf(nil) = %d, want %d", got, cliexit.CodeSuccess)
	}
}

func TestExitCodeOf_CliError(t *testing.T) {
	err := cliexit.NewAuth(cliexit.ErrCodeAuthNoBinary, errors.New("gh not found"))
	if got := cliexit.ExitCodeOf(err); got != cliexit.CodeAuth {
		t.Errorf("ExitCodeOf(auth) = %d, want %d", got, cliexit.CodeAuth)
	}
}

func TestExitCodeOf_Wrapped(t *testing.T) {
	inner := cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs, errors.New("bad flag"))
	wrapped := errors.New("outer: " + inner.Error())
	// plain error → CodeGeneral
	if got := cliexit.ExitCodeOf(wrapped); got != cliexit.CodeGeneral {
		t.Errorf("ExitCodeOf(plain wrap) = %d, want %d", got, cliexit.CodeGeneral)
	}
}

func TestExitCodeOf_ErrorsAs(t *testing.T) {
	inner := cliexit.NewValidation(cliexit.ErrCodeValidation, errors.New("invalid"), nil)
	// wrap with fmt.Errorf %w so errors.As can unwrap
	wrapped := errors.Join(inner, errors.New("extra"))
	if got := cliexit.ExitCodeOf(wrapped); got != cliexit.CodeValidation {
		t.Errorf("ExitCodeOf(joined) = %d, want %d", got, cliexit.CodeValidation)
	}
}

// ─── Render: plain text ───────────────────────────────────────────────────────

func TestRender_PlainText(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cliexit.NewGeneral(errors.New("something went wrong"))
	cliexit.Render(err, false, &stdout, &stderr)

	if stdout.Len() != 0 {
		t.Errorf("plain: unexpected stdout %q", stdout.String())
	}
	want := "error: something went wrong\n"
	if got := stderr.String(); got != want {
		t.Errorf("plain stderr = %q, want %q", got, want)
	}
}

func TestRender_PlainText_Nil(t *testing.T) {
	var stdout, stderr bytes.Buffer
	cliexit.Render(nil, false, &stdout, &stderr)
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Error("Render(nil) should produce no output")
	}
}

// ─── Render: JSON ─────────────────────────────────────────────────────────────

func TestRender_JSON_Structure(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cliexit.NewAuth(cliexit.ErrCodeAuthNoBinary, errors.New("gh not found"))
	cliexit.Render(err, true, &stdout, &stderr)

	if stderr.Len() != 0 {
		t.Errorf("json: unexpected stderr %q", stderr.String())
	}

	var got struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if parseErr := json.Unmarshal(stdout.Bytes(), &got); parseErr != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", parseErr, stdout.String())
	}
	if got.Error.Code != string(cliexit.ErrCodeAuthNoBinary) {
		t.Errorf("json code = %q, want %q", got.Error.Code, cliexit.ErrCodeAuthNoBinary)
	}
	if got.Error.Message != "gh not found" {
		t.Errorf("json message = %q, want %q", got.Error.Message, "gh not found")
	}
}

func TestRender_JSON_WithDetails(t *testing.T) {
	var stdout, stderr bytes.Buffer
	details := map[string]any{"field": "name", "issue": "required"}
	err := cliexit.NewValidation(cliexit.ErrCodeValidation, errors.New("validation failed"), details)
	cliexit.Render(err, true, &stdout, &stderr)

	var got struct {
		Error struct {
			Details map[string]any `json:"details"`
		} `json:"error"`
	}
	if parseErr := json.Unmarshal(stdout.Bytes(), &got); parseErr != nil {
		t.Fatalf("parse error: %v", parseErr)
	}
	if got.Error.Details["field"] != "name" {
		t.Errorf("details.field = %v, want %q", got.Error.Details["field"], "name")
	}
}

// ─── Constructors: exit codes ────────────────────────────────────────────────

func TestConstructors_ExitCodes(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"NewNotFound", cliexit.NewNotFound(cliexit.ErrCodeNotFound, errors.New("x")), cliexit.CodeNotFound},
		{"NewAuth", cliexit.NewAuth(cliexit.ErrCodeAuthNoBinary, errors.New("x")), cliexit.CodeAuth},
		{"NewUsage", cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs, errors.New("x")), cliexit.CodeUsage},
		{"NewValidation", cliexit.NewValidation(cliexit.ErrCodeValidation, errors.New("x"), nil), cliexit.CodeValidation},
		{"NewAPI", cliexit.NewAPI(cliexit.ErrCodeAPI, errors.New("x")), cliexit.CodeAPI},
		{"NewGeneral", cliexit.NewGeneral(errors.New("x")), cliexit.CodeGeneral},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := cliexit.ExitCodeOf(tc.err); got != tc.want {
				t.Errorf("ExitCodeOf = %d, want %d", got, tc.want)
			}
		})
	}
}

// ─── Render: JSON category / exit_code / next_step ───────────────────────────

func TestRender_JSON_Category(t *testing.T) {
	cases := []struct {
		name         string
		err          error
		wantCategory string
		wantExitCode int
	}{
		{"auth", cliexit.NewAuth(cliexit.ErrCodeAuthNoBinary, errors.New("x")), "auth", cliexit.CodeAuth},
		{"not_found", cliexit.NewNotFound(cliexit.ErrCodeNotFound, errors.New("x")), "not_found", cliexit.CodeNotFound},
		{"validation", cliexit.NewValidation(cliexit.ErrCodeValidation, errors.New("x"), nil), "validation", cliexit.CodeValidation},
		{"api", cliexit.NewAPI(cliexit.ErrCodeAPI, errors.New("x")), "api", cliexit.CodeAPI},
		{"usage", cliexit.NewUsage(cliexit.ErrCodeUsageBadArgs, errors.New("x")), "usage", cliexit.CodeUsage},
		{"general", cliexit.NewGeneral(errors.New("x")), "general", cliexit.CodeGeneral},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			cliexit.Render(tc.err, true, &stdout, &stderr)

			var got struct {
				Error struct {
					Category string `json:"category"`
					ExitCode int    `json:"exit_code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
				t.Fatalf("parse JSON: %v\nraw: %s", err, stdout.String())
			}
			if got.Error.Category != tc.wantCategory {
				t.Errorf("category = %q, want %q", got.Error.Category, tc.wantCategory)
			}
			if got.Error.ExitCode != tc.wantExitCode {
				t.Errorf("exit_code = %d, want %d", got.Error.ExitCode, tc.wantExitCode)
			}
		})
	}
}

func TestRender_JSON_NextStep(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := cliexit.NewAuth(cliexit.ErrCodeAuthNoToken, errors.New("not authenticated"))
	err.NextStep = "Run: gh auth login"
	cliexit.Render(err, true, &stdout, &stderr)

	var got struct {
		Error struct {
			NextStep string `json:"next_step"`
		} `json:"error"`
	}
	if parseErr := json.Unmarshal(stdout.Bytes(), &got); parseErr != nil {
		t.Fatalf("parse JSON: %v\nraw: %s", parseErr, stdout.String())
	}
	if got.Error.NextStep != "Run: gh auth login" {
		t.Errorf("next_step = %q, want %q", got.Error.NextStep, "Run: gh auth login")
	}
}
