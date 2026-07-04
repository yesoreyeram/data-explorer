package nodes

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

func StreamCSV(r io.Reader, maxRows int) (*dataframe.Frame, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err == io.EOF {
		return dataframe.New(nil), nil
	}
	if err != nil {
		return nil, fmt.Errorf("parse csv: %w", err)
	}
	frame := dataframe.New(nil)
	truncated := false
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse csv: %w", err)
		}
		if maxRows > 0 && frame.NumRows() >= maxRows {
			truncated = true
			break
		}
		row := make(map[string]any, len(header))
		for i, col := range header {
			if i < len(record) {
				row[col] = record[i]
			} else {
				row[col] = nil
			}
		}
		frame.AppendRow(row)
	}
	if truncated {
		frame.Meta.Truncated = true
		frame.Meta = frame.Meta.WithWarning("Row cap reached while streaming CSV input.")
	}
	return frame, nil
}

func StreamNDJSON(r io.Reader, maxRows int) (*dataframe.Frame, error) {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 8*1024*1024)
	frame := dataframe.New(nil)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		if maxRows > 0 && frame.NumRows() >= maxRows {
			frame.Meta.Truncated = true
			frame.Meta = frame.Meta.WithWarning("Row cap reached while streaming NDJSON input.")
			return frame, nil
		}
		var row map[string]any
		dec := json.NewDecoder(strings.NewReader(text))
		dec.UseNumber()
		if err := dec.Decode(&row); err != nil {
			return nil, fmt.Errorf("parse ndjson line %d: %w", line, err)
		}
		frame.AppendRow(row)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse ndjson: %w", err)
	}
	return frame, nil
}
