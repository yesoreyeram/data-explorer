package safejson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// Unmarshal decodes JSON with depth and element limits.
func Unmarshal(data []byte, v any, maxDepth, maxElements int) error {
	if err := validate(data, maxDepth, maxElements); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if err := ensureEOF(dec); err != nil {
		return err
	}
	return nil
}

func validate(data []byte, maxDepth, maxElements int) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	depth := 0
	elements := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		elements++
		if maxElements > 0 && elements > maxElements {
			return fmt.Errorf("json exceeds the %d token limit", maxElements)
		}
		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '{', '[':
				depth++
				if maxDepth > 0 && depth > maxDepth {
					return fmt.Errorf("json exceeds the maximum depth of %d", maxDepth)
				}
			case '}', ']':
				if depth > 0 {
					depth--
				}
			}
		}
	}
}

func ensureEOF(dec *json.Decoder) error {
	if _, err := dec.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("invalid trailing JSON data")
		}
		return err
	}
	return nil
}
