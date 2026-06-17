// Package cliexit provides structured error types and exit code conventions
// for gh-wheel subcommands.
//
// Exit codes:
//
//	0 — success
//	1 — general / internal error
//	2 — usage error (invalid args, unknown flag, …)
//	3 — resource not found
//	4 — authentication error
//	5 — validation error
//	6 — GitHub API error
package cliexit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Process exit code constants.
const (
	CodeSuccess    = 0
	CodeGeneral    = 1
	CodeUsage      = 2
	CodeNotFound   = 3
	CodeAuth       = 4
	CodeValidation = 5
	CodeAPI        = 6
)

// ErrCode is a machine-readable error identifier embedded in JSON output.
type ErrCode string

const (
	// Not found
	ErrCodeNotFound ErrCode = "NOT_FOUND"

	// Auth
	ErrCodeAuthNoBinary ErrCode = "AUTH_GH_NOT_FOUND"
	ErrCodeAuthNoToken  ErrCode = "AUTH_NOT_LOGGED_IN"
	ErrCodeAuthOldGH    ErrCode = "AUTH_GH_VERSION_TOO_OLD"

	// Usage
	ErrCodeUsageBadArgs ErrCode = "USAGE_INVALID_ARGS"
	ErrCodeUsageNoRepo  ErrCode = "USAGE_REPO_NOT_DETECTED"

	// Validation
	ErrCodeValidation ErrCode = "VALIDATION_FAILED"

	// API
	ErrCodeAPI ErrCode = "API_REQUEST_FAILED"

	// General
	ErrCodeGeneral ErrCode = "INTERNAL_ERROR"
)

// Error is a structured gh-wheel error that carries a process exit code,
// a machine-readable code, a human-readable message, optional structured
// details, a next-step hint for the operator, and a wrapped inner error.
type Error struct {
	ExitCode int
	Code     ErrCode
	Message  string
	Details  map[string]any
	NextStep string
	Wrapped  error
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Wrapped }

// ─── Constructors ─────────────────────────────────────────────────────────────

// NewNotFound returns a not-found error (exit code 3).
func NewNotFound(code ErrCode, err error) *Error {
	return &Error{ExitCode: CodeNotFound, Code: code, Message: errMsg(err), Wrapped: err}
}

// NewAuth returns an authentication error (exit code 4).
func NewAuth(code ErrCode, err error) *Error {
	return &Error{ExitCode: CodeAuth, Code: code, Message: errMsg(err), Wrapped: err}
}

// NewUsage returns a usage / invalid-argument error (exit code 2).
func NewUsage(code ErrCode, err error) *Error {
	return &Error{ExitCode: CodeUsage, Code: code, Message: errMsg(err), Wrapped: err}
}

// NewValidation returns a validation error (exit code 5).
// details may be nil.
func NewValidation(code ErrCode, err error, details map[string]any) *Error {
	return &Error{ExitCode: CodeValidation, Code: code, Message: errMsg(err), Details: details, Wrapped: err}
}

// NewAPI returns a GitHub API error (exit code 6).
func NewAPI(code ErrCode, err error) *Error {
	return &Error{ExitCode: CodeAPI, Code: code, Message: errMsg(err), Wrapped: err}
}

// NewGeneral returns a generic internal error (exit code 1).
func NewGeneral(err error) *Error {
	return &Error{ExitCode: CodeGeneral, Code: ErrCodeGeneral, Message: errMsg(err), Wrapped: err}
}

// ─── Rendering ────────────────────────────────────────────────────────────────

// Render writes err to the appropriate writer.
//
// When asJSON is true:
//   - success result: caller prints JSON; errors go to stdout as
//     {"error":{"code","message","details?"}} so scripts can parse them.
//
// When asJSON is false:
//   - errors go to stderr as "error: <message>\n".
//
// Non-*Error values are wrapped in NewGeneral so they get a structured code.
func Render(err error, asJSON bool, stdout, stderr io.Writer) {
	if err == nil {
		return
	}
	var e *Error
	if !errors.As(err, &e) {
		e = NewGeneral(err)
	}
	if asJSON {
		out := struct {
			Error struct {
				Category string         `json:"category"`
				ExitCode int            `json:"exit_code"`
				Code     ErrCode        `json:"code"`
				Message  string         `json:"message"`
				NextStep string         `json:"next_step,omitempty"`
				Details  map[string]any `json:"details,omitempty"`
			} `json:"error"`
		}{}
		out.Error.Category = categoryOf(e)
		out.Error.ExitCode = e.ExitCode
		out.Error.Code = e.Code
		out.Error.Message = e.Message
		out.Error.NextStep = e.NextStep
		out.Error.Details = e.Details
		b, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			// Fallback if Details contains unmarshalable values.
			fmt.Fprintf(stderr, "error: %s\n", e.Message)
			return
		}
		fmt.Fprintf(stdout, "%s\n", b)
		return
	}
	fmt.Fprintf(stderr, "error: %s\n", e.Message)
}

// ExitCodeOf returns the appropriate process exit code for err.
// It walks the error chain with errors.As to find the first (outermost) *Error.
// Returns CodeSuccess for nil, CodeGeneral for non-*Error errors.
func ExitCodeOf(err error) int {
	if err == nil {
		return CodeSuccess
	}
	var e *Error
	if errors.As(err, &e) {
		return e.ExitCode
	}
	return CodeGeneral
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func categoryOf(e *Error) string {
	switch e.ExitCode {
	case CodeNotFound:
		return "not_found"
	case CodeAuth:
		return "auth"
	case CodeValidation:
		return "validation"
	case CodeAPI:
		return "api"
	case CodeUsage:
		return "usage"
	default:
		return "general"
	}
}

func errMsg(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
