package dataframe

import "testing"

func TestTruncateCells(t *testing.T) {
	f := FromRecords([]map[string]any{{"blob": "0123456789"}})

	affected := f.TruncateCells(5)
	if affected != 1 {
		t.Fatalf("expected 1 cell truncated, got %d", affected)
	}
	got := f.Rows()[0]["blob"].(string)
	if len(got) <= 5 {
		t.Fatalf("expected truncated marker appended, got %q", got)
	}
	if len(f.Meta.Warnings) != 1 {
		t.Fatalf("expected a warning recorded, got %v", f.Meta.Warnings)
	}
}

func TestLimitRowsMarksTruncated(t *testing.T) {
	f := FromRecords([]map[string]any{{"n": int64(1)}, {"n": int64(2)}, {"n": int64(3)}})

	f.LimitRows(2)
	if f.NumRows() != 2 {
		t.Fatalf("expected 2 rows after limit, got %d", f.NumRows())
	}
	if !f.Meta.Truncated {
		t.Fatal("expected Meta.Truncated to be set")
	}
}

func TestLimitRowsNoopWhenUnderLimit(t *testing.T) {
	f := FromRecords([]map[string]any{{"n": int64(1)}})
	f.LimitRows(10)
	if f.Meta.Truncated {
		t.Fatal("did not expect Truncated to be set when under the limit")
	}
}
