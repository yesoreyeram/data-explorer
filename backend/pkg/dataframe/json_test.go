package dataframe

import (
	"encoding/json"
	"testing"
)

func TestFrameJSONRoundTrip(t *testing.T) {
	f := FromRecords([]map[string]any{
		{"id": int64(1), "name": "alice"},
		{"id": int64(2), "name": "bob"},
	})
	f.SetMeta(Metadata{Name: "users", SourceType: "postgres"})

	data, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var out Frame
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	if out.NumRows() != 2 {
		t.Fatalf("expected 2 rows after round-trip, got %d", out.NumRows())
	}
	if out.Meta.Name != "users" || out.Meta.SourceType != "postgres" {
		t.Fatalf("expected metadata to survive round-trip, got %+v", out.Meta)
	}
	if got := out.Rows()[1]["name"]; got != "bob" {
		t.Fatalf("expected row 1 name=bob, got %v", got)
	}
}

func TestFrameJSONIncludesSchema(t *testing.T) {
	f := FromRecords([]map[string]any{{"active": true}})
	data, _ := json.Marshal(f)

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal into map: %v", err)
	}
	schema, ok := decoded["schema"].(map[string]any)
	if !ok {
		t.Fatalf("expected a schema object in wire format, got %+v", decoded)
	}
	fields, ok := schema["fields"].([]any)
	if !ok || len(fields) != 1 {
		t.Fatalf("expected one field in schema, got %+v", schema)
	}
}
