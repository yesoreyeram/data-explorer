package dataframe

import (
	"fmt"
	"time"
)

// Metadata carries provenance and quality information about a Frame,
// separate from the data itself: where it came from, how long it took to
// produce, whether it was truncated by a guardrail, and any non-fatal
// warnings collected along the way. This is what lets a workflow execution
// report ("node X read 1,000 rows from connection Y in 42ms, truncated")
// without the caller having to thread that information through side
// channels.
type Metadata struct {
	// Name is a short human label, e.g. the node name or connection name.
	Name string `json:"name,omitempty"`
	// SourceType identifies what produced the frame, e.g. "postgres",
	// "mysql", "rest", "graphql", "node:transform", "node:join".
	SourceType string `json:"sourceType,omitempty"`
	// SourceID identifies the specific origin, e.g. a connection ID or node ID.
	SourceID string `json:"sourceId,omitempty"`
	// Lineage records the chain of frames this one was derived from (source
	// IDs/labels of upstream frames), oldest first, for debugging pipelines.
	Lineage []string `json:"lineage,omitempty"`

	GeneratedAt time.Time `json:"generatedAt"`
	DurationMs  int64     `json:"durationMs"`

	RowCount    int `json:"rowCount"`
	ColumnCount int `json:"columnCount"`

	// Truncated is true when a guardrail (row limit, pagination cap, byte
	// cap, ...) cut the result short of the source's true size.
	Truncated bool `json:"truncated"`
	// Warnings are non-fatal issues surfaced to the caller (e.g. "3 cells
	// exceeded the max cell size and were truncated").
	Warnings []string `json:"warnings,omitempty"`

	// Extra holds source-specific detail that doesn't warrant a first-class
	// field (e.g. {"pagesFetched": 4} for a paginated REST call).
	Extra map[string]any `json:"extra,omitempty"`
}

// WithWarning appends a warning and returns the metadata for chaining.
func (m Metadata) WithWarning(format string, args ...any) Metadata {
	if len(args) == 0 {
		m.Warnings = append(m.Warnings, format)
	} else {
		m.Warnings = append(m.Warnings, fmt.Sprintf(format, args...))
	}
	return m
}
