package jsonout_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/UtakataKyosui/gh-wheel/internal/jsonout"
)

type testPayload struct {
	Name  string   `json:"name"`
	Items []string `json:"items"`
}

func TestWrite_ValidJSON(t *testing.T) {
	var buf bytes.Buffer
	p := testPayload{Name: "hello", Items: []string{"a", "b"}}
	if err := jsonout.Write(&buf, p, ""); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	var got testPayload
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("parse output: %v\nraw: %s", err, buf.String())
	}
	if got.Name != "hello" {
		t.Errorf("name = %q, want %q", got.Name, "hello")
	}
}

// TestWrite_NonNilSliceIsArray verifies that non-nil slices serialize as JSON
// arrays. Nil-slice normalisation (null→[]) is the caller's responsibility.
func TestWrite_NonNilSliceIsArray(t *testing.T) {
	var buf bytes.Buffer
	type S struct {
		Items []string `json:"items"`
	}
	// Caller explicitly sets empty (non-nil) slice.
	if err := jsonout.Write(&buf, S{Items: []string{}}, ""); err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if !strings.Contains(buf.String(), `"items": []`) {
		t.Errorf("empty slice should be [], got: %s", buf.String())
	}
}

func TestWrite_Indented(t *testing.T) {
	var buf bytes.Buffer
	p := testPayload{Name: "x", Items: []string{"y"}}
	if err := jsonout.Write(&buf, p, ""); err != nil {
		t.Fatal(err)
	}
	// Indented JSON contains newlines.
	if !strings.Contains(buf.String(), "\n") {
		t.Errorf("expected indented output with newlines, got: %s", buf.String())
	}
}

// TestWrite_JqFilter_String verifies that jq string results are written as raw
// strings (no JSON quoting), matching gh's built-in --jq behaviour.
func TestWrite_JqFilter_String(t *testing.T) {
	var buf bytes.Buffer
	p := testPayload{Name: "hello", Items: []string{"a", "b"}}
	if err := jsonout.Write(&buf, p, ".name"); err != nil {
		t.Fatalf("Write with jq error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	// go-gh's jq.Evaluate outputs raw strings (no surrounding "...") for
	// scalar string results.
	if got != "hello" {
		t.Errorf("jq .name = %q, want %q", got, "hello")
	}
}

func TestWrite_JqFilter_Array(t *testing.T) {
	var buf bytes.Buffer
	p := testPayload{Name: "hello", Items: []string{"a", "b"}}
	if err := jsonout.Write(&buf, p, ".items | length"); err != nil {
		t.Fatalf("Write with jq error: %v", err)
	}
	got := strings.TrimSpace(buf.String())
	if got != "2" {
		t.Errorf("jq .items | length = %q, want %q", got, "2")
	}
}
