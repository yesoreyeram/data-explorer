package connectors

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
)

// InferObjectFormat guesses a data format from an object key's extension,
// for the common case where the caller doesn't specify one explicitly.
// Defaults to "json" when the extension is unrecognized.
func InferObjectFormat(key string) string {
	lower := strings.ToLower(key)
	switch {
	case strings.HasSuffix(lower, ".csv"):
		return "csv"
	case strings.HasSuffix(lower, ".ndjson"), strings.HasSuffix(lower, ".jsonl"):
		return "ndjson"
	default:
		return "json"
	}
}

// ParseObjectRows decodes a cloud storage object's bytes (S3 / GCS / Azure
// Blob Storage) into rows, sharing one implementation across all three
// object storage connectors. format is one of "csv"/"json"/"ndjson"
// (resolved via InferObjectFormat by the caller if the user didn't specify
// one); delimiter defaults to "," for CSV.
func ParseObjectRows(data []byte, format, delimiter string) ([]map[string]any, error) {
	switch format {
	case "csv":
		return parseCSVRows(data, delimiter)
	case "ndjson":
		return parseNDJSONRows(data)
	case "json", "":
		return parseJSONRows(data)
	default:
		return nil, fmt.Errorf("unsupported object format %q", format)
	}
}

func parseCSVRows(data []byte, delimiter string) ([]map[string]any, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1 // tolerate ragged rows rather than failing the whole file
	if delimiter != "" {
		runes := []rune(delimiter)
		reader.Comma = runes[0]
	}

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse csv: %w", err)
	}
	if len(records) == 0 {
		return nil, nil
	}

	header := records[0]
	rows := make([]map[string]any, 0, len(records)-1)
	for _, record := range records[1:] {
		row := make(map[string]any, len(header))
		for i, col := range header {
			if i < len(record) {
				row[col] = record[i]
			} else {
				row[col] = nil
			}
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func parseNDJSONRows(data []byte) ([]map[string]any, error) {
	var rows []map[string]any
	for i, line := range bytes.Split(data, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var row map[string]any
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, fmt.Errorf("parse ndjson line %d: %w", i+1, err)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func parseJSONRows(data []byte) ([]map[string]any, error) {
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	return toRowMaps(decoded), nil
}

// toRowMaps normalizes an arbitrary decoded JSON value into rows: an array
// is used as-is, a single object becomes a one-item slice.
func toRowMaps(decoded any) []map[string]any {
	switch v := decoded.(type) {
	case []any:
		rows := make([]map[string]any, 0, len(v))
		for _, item := range v {
			rows = append(rows, toRowMap(item))
		}
		return rows
	case nil:
		return nil
	default:
		return []map[string]any{toRowMap(v)}
	}
}
