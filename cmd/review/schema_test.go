package review

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestSchemaCmd_OutputsValidJSON(t *testing.T) {
	cmd := NewSchemaCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("schema cmd execute error: %v", err)
	}
	if !json.Valid(buf.Bytes()) {
		t.Errorf("schema cmd output is not valid JSON: %s", buf.String())
	}
}
