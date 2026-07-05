package connectors

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"strings"

	"github.com/yesoreyeram/data-explorer/backend/internal/platform/safejson"
	workflownodes "github.com/yesoreyeram/data-explorer/backend/internal/workflow/nodes"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

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

func ParseObjectRows(data []byte, format, delimiter string) ([]map[string]any, error) {
	frame, err := ParseObjectFrame(bytes.NewReader(data), format, delimiter, 0)
	if err != nil {
		return nil, err
	}
	return frame.Rows(), nil
}

func ParseObjectFrame(r io.Reader, format, delimiter string, maxRows int) (*dataframe.Frame, error) {
	switch format {
	case "csv":
		if delimiter != "" {
			reader := csv.NewReader(r)
			reader.FieldsPerRecord = -1
			runes := []rune(delimiter)
			reader.Comma = runes[0]
			records, err := reader.ReadAll()
			if err != nil {
				return nil, fmt.Errorf("parse csv: %w", err)
			}
			if len(records) == 0 {
				return dataframe.New(nil), nil
			}
			rows := make([]map[string]any, 0, len(records)-1)
			header := records[0]
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
			frame := dataframe.FromRecords(rows)
			if maxRows > 0 {
				frame.LimitRows(maxRows)
			}
			return frame, nil
		}
		return workflownodes.StreamCSV(r, maxRows)
	case "ndjson":
		return workflownodes.StreamNDJSON(r, maxRows)
	case "json", "":
		data, err := io.ReadAll(r)
		if err != nil {
			return nil, err
		}
		var decoded any
		if err := safejson.Unmarshal(data, &decoded, 64, 5_000_000); err != nil {
			return nil, fmt.Errorf("parse json: %w", err)
		}
		frame := dataframe.FromRecords(toRowMaps(decoded))
		if maxRows > 0 {
			frame.LimitRows(maxRows)
		}
		return frame, nil
	default:
		return nil, fmt.Errorf("unsupported object format %q", format)
	}
}

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
