package reviewschema

var schemaJSON = []byte(`{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "$id": "gh-wheel/review/v1",
  "type": "object",
  "additionalProperties": false,
  "required": ["event", "summary", "comments"],
  "properties": {
    "event": { "type": "string", "enum": ["COMMENT", "REQUEST_CHANGES", "APPROVE"] },
    "summary": { "type": "string", "minLength": 1 },
    "comments": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["path", "line", "body"],
        "properties": {
          "path": { "type": "string" },
          "line": { "type": "integer", "minimum": 1 },
          "side": { "type": "string", "enum": ["RIGHT", "LEFT"], "default": "RIGHT" },
          "start_line": { "type": "integer" },
          "body": { "type": "string" },
          "suggestion": { "type": "string" },
          "skip_suggestion": { "type": "boolean" },
          "reason": { "type": "string" }
        }
      }
    }
  }
}`)

// Schema returns the JSON Schema for gh-wheel review output (v1).
func Schema() []byte {
	return schemaJSON
}
