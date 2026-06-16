package reviewschema_test

import (
	"encoding/json"
	"testing"

	"github.com/UtakataKyosui/gh-wheel/internal/reviewschema"
)

func TestSchema_IsValidJSON(t *testing.T) {
	b := reviewschema.Schema()
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("Schema() is not valid JSON: %v", err)
	}
}

func TestSchema_HasID(t *testing.T) {
	b := reviewschema.Schema()
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("Schema() is not valid JSON: %v", err)
	}
	id, ok := m["$id"].(string)
	if !ok {
		t.Fatal("Schema() missing $id field")
	}
	if id != "gh-wheel/review/v1" {
		t.Errorf("$id = %q, want %q", id, "gh-wheel/review/v1")
	}
}
