package dataframe

import "testing"

func TestFromRecordsInfersSchemaAndOrder(t *testing.T) {
	f := FromRecords([]map[string]any{
		{"name": "alice", "age": int64(30)},
		{"name": "bob", "age": int64(25)},
	})

	if f.NumRows() != 2 {
		t.Fatalf("expected 2 rows, got %d", f.NumRows())
	}
	if got := f.ColumnNames(); len(got) != 2 || got[0] != "name" || got[1] != "age" {
		t.Fatalf("expected column order [name age], got %v", got)
	}
	ageField, ok := f.Schema().FieldByName("age")
	if !ok || ageField.Type != TypeInt64 {
		t.Fatalf("expected age:int64, got %+v (ok=%v)", ageField, ok)
	}
}

func TestAppendRowWidensTypeAndBackfillsNulls(t *testing.T) {
	f := New(nil)
	f.AppendRow(map[string]any{"id": int64(1)})
	f.AppendRow(map[string]any{"id": int64(2), "score": 1.5})      // new column mid-stream
	f.AppendRow(map[string]any{"id": int64(3), "score": int64(4)}) // widens score to float64

	scoreField, _ := f.Schema().FieldByName("score")
	if scoreField.Type != TypeFloat64 {
		t.Fatalf("expected score to widen to float64, got %s", scoreField.Type)
	}
	if !scoreField.Nullable {
		t.Fatal("expected score to be nullable (row 0 had no score)")
	}

	rows := f.Rows()
	if rows[0]["score"] != nil {
		t.Fatalf("expected row 0 score to be backfilled nil, got %v", rows[0]["score"])
	}
}

func TestSelectAndRename(t *testing.T) {
	f := FromRecords([]map[string]any{{"a": int64(1), "b": int64(2)}})

	selected := f.Select("b")
	if selected.NumCols() != 1 || selected.ColumnNames()[0] != "b" {
		t.Fatalf("expected only column b, got %v", selected.ColumnNames())
	}

	renamed := f.Rename(map[string]string{"a": "alpha"})
	if _, ok := renamed.Column("alpha"); !ok {
		t.Fatal("expected renamed column 'alpha' to exist")
	}
	if _, ok := renamed.Column("a"); ok {
		t.Fatal("expected old column name 'a' to be gone")
	}
}

func TestFilter(t *testing.T) {
	f := FromRecords([]map[string]any{
		{"amount": int64(150)},
		{"amount": int64(40)},
		{"amount": int64(200)},
	})

	out := f.Filter(func(_ int, row map[string]any) bool {
		v, _ := toFloat(row["amount"])
		return v > 100
	})

	if out.NumRows() != 2 {
		t.Fatalf("expected 2 rows after filter, got %d", out.NumRows())
	}
}

func TestConcatUnionsSchemas(t *testing.T) {
	a := FromRecords([]map[string]any{{"x": int64(1)}})
	b := FromRecords([]map[string]any{{"x": int64(2), "y": "new"}})

	out := Concat("combined", a, b)
	if out.NumRows() != 2 {
		t.Fatalf("expected 2 rows, got %d", out.NumRows())
	}
	if _, ok := out.Schema().FieldByName("y"); !ok {
		t.Fatal("expected unioned schema to include column y")
	}
	rows := out.Rows()
	if rows[0]["y"] != nil {
		t.Fatalf("expected first row's y to be nil (not present in frame a), got %v", rows[0]["y"])
	}
}
