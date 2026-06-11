// Package jsonout serialises arbitrary values to indented JSON and optionally
// filters the output through a jq expression using the bundled gojq engine
// from go-gh (github.com/cli/go-gh/v2/pkg/jq).
//
// Nil-slice normalisation is the caller's responsibility: set nil slices to
// empty slices before calling Write so JSON output shows [] rather than null.
package jsonout

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/cli/go-gh/v2/pkg/jq"
)

// Write marshals v to indented JSON and writes it to w.
//
// If jqExpr is non-empty, the JSON is piped through the expression using the
// bundled gojq engine — no external jq binary is required.
// String results from jq expressions are written as raw strings (no JSON
// quoting), matching the behaviour of `gh`'s built-in --jq flag.
func Write(w io.Writer, v any, jqExpr string) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}

	if jqExpr == "" {
		_, err = fmt.Fprintf(w, "%s\n", b)
		return err
	}

	// Filter through gojq (bundled with go-gh, no external dependency).
	// go-gh's jq.Evaluate outputs raw strings for scalar string results
	// (equivalent to jq -r for strings, jq for objects/arrays).
	return jq.Evaluate(bytes.NewReader(b), w, jqExpr)
}

// Print is a convenience wrapper that writes to os.Stdout.
func Print(v any, jqExpr string) error {
	return Write(os.Stdout, v, jqExpr)
}
