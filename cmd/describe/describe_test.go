package describe_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/UtakataKyosui/gh-wheel/cmd/describe"
)

func TestNewCmd_Basic(t *testing.T) {
	cmd := describe.NewCmd()
	if cmd.Use != "describe" {
		t.Errorf("Use: want %q, got %q", "describe", cmd.Use)
	}
	if cmd.Short == "" {
		t.Error("Short should not be empty")
	}
	if err := cmd.Args(cmd, []string{"unexpected"}); err == nil {
		t.Error("command should reject positional arguments")
	}
}

func TestWrite_SchemaVersion(t *testing.T) {
	var buf bytes.Buffer
	if err := describe.Write(&buf); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("parse JSON: %v\nraw: %s", err, buf.String())
	}

	if got["schema_version"] != "v1" {
		t.Errorf("schema_version = %v, want %q", got["schema_version"], "v1")
	}
	if got["kind"] != "command_schema" {
		t.Errorf("kind = %v, want %q", got["kind"], "command_schema")
	}
	if got["name"] != "wheel" {
		t.Errorf("name = %v, want %q", got["name"], "wheel")
	}
}

func TestWrite_Commands(t *testing.T) {
	var buf bytes.Buffer
	if err := describe.Write(&buf); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	var got struct {
		Commands []struct {
			Name string `json:"name"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if len(got.Commands) == 0 {
		t.Error("commands list should not be empty")
	}
	names := make(map[string]bool, len(got.Commands))
	for _, c := range got.Commands {
		names[c.Name] = true
	}
	for _, want := range []string{"task", "task next", "task today", "graph", "monitor", "review", "okr", "okr metrics", "feedback", "describe"} {
		if !names[want] {
			t.Errorf("command %q not found in describe output", want)
		}
	}
}

func TestWrite_ExitCodes(t *testing.T) {
	var buf bytes.Buffer
	if err := describe.Write(&buf); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	var got struct {
		ExitCodes []struct {
			Code     int    `json:"code"`
			Category string `json:"category"`
		} `json:"exit_codes"`
	}
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if len(got.ExitCodes) == 0 {
		t.Error("exit_codes list should not be empty")
	}
	categories := make(map[string]bool)
	for _, ec := range got.ExitCodes {
		categories[ec.Category] = true
	}
	for _, want := range []string{"success", "general", "usage", "not_found", "auth", "validation", "api"} {
		if !categories[want] {
			t.Errorf("exit_code category %q not found in describe output", want)
		}
	}
}
