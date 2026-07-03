package connectors

import "testing"

func TestInferObjectFormat(t *testing.T) {
	cases := map[string]string{
		"data.csv":         "csv",
		"logs/file.NDJSON": "ndjson",
		"events.jsonl":     "ndjson",
		"payload.json":     "json",
		"no-extension":     "json",
	}
	for key, want := range cases {
		if got := InferObjectFormat(key); got != want {
			t.Errorf("InferObjectFormat(%q) = %q, want %q", key, got, want)
		}
	}
}

func TestParseObjectRowsCSV(t *testing.T) {
	data := []byte("id,name\n1,alice\n2,bob\n")
	rows, err := ParseObjectRows(data, "csv", "")
	if err != nil {
		t.Fatalf("ParseObjectRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["name"] != "alice" {
		t.Fatalf("expected row 0 name=alice, got %+v", rows[0])
	}
}

func TestParseObjectRowsCSVCustomDelimiter(t *testing.T) {
	data := []byte("id;name\n1;alice\n")
	rows, err := ParseObjectRows(data, "csv", ";")
	if err != nil {
		t.Fatalf("ParseObjectRows: %v", err)
	}
	if rows[0]["name"] != "alice" {
		t.Fatalf("expected row 0 name=alice, got %+v", rows[0])
	}
}

func TestParseObjectRowsJSONArray(t *testing.T) {
	data := []byte(`[{"id":1},{"id":2}]`)
	rows, err := ParseObjectRows(data, "json", "")
	if err != nil {
		t.Fatalf("ParseObjectRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
}

func TestParseObjectRowsJSONSingleObject(t *testing.T) {
	data := []byte(`{"id":1,"name":"solo"}`)
	rows, err := ParseObjectRows(data, "json", "")
	if err != nil {
		t.Fatalf("ParseObjectRows: %v", err)
	}
	if len(rows) != 1 || rows[0]["name"] != "solo" {
		t.Fatalf("expected a single wrapped row, got %+v", rows)
	}
}

func TestParseObjectRowsNDJSON(t *testing.T) {
	data := []byte("{\"id\":1}\n{\"id\":2}\n\n")
	rows, err := ParseObjectRows(data, "ndjson", "")
	if err != nil {
		t.Fatalf("ParseObjectRows: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (blank line skipped), got %d", len(rows))
	}
}

func TestParseObjectRowsUnsupportedFormat(t *testing.T) {
	if _, err := ParseObjectRows([]byte("x"), "parquet", ""); err == nil {
		t.Fatal("expected an error for an unsupported format")
	}
}
