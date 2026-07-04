package dataframe

import "testing"

func TestJoinInner(t *testing.T) {
	left := FromRecords([]map[string]any{
		{"id": int64(1), "name": "alice"},
		{"id": int64(2), "name": "bob"},
	})
	right := FromRecords([]map[string]any{
		{"user_id": int64(1), "role": "admin"},
	})

	out, err := Join(left, right, JoinOptions{LeftKey: "id", RightKey: "user_id", Type: JoinInner})
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	if out.NumRows() != 1 {
		t.Fatalf("expected 1 matched row, got %d", out.NumRows())
	}
	if out.Rows()[0]["role"] != "admin" {
		t.Fatalf("expected role=admin, got %+v", out.Rows()[0])
	}
}

func TestJoinLeftKeepsUnmatched(t *testing.T) {
	left := FromRecords([]map[string]any{
		{"id": int64(1)},
		{"id": int64(2)},
	})
	right := FromRecords([]map[string]any{
		{"user_id": int64(1), "role": "admin"},
	})

	out, err := Join(left, right, JoinOptions{LeftKey: "id", RightKey: "user_id", Type: JoinLeft})
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	if out.NumRows() != 2 {
		t.Fatalf("expected 2 rows (one unmatched), got %d", out.NumRows())
	}
}

func TestJoinPrefixesCollidingColumns(t *testing.T) {
	left := FromRecords([]map[string]any{{"id": int64(1), "name": "left-name"}})
	right := FromRecords([]map[string]any{{"id": int64(1), "name": "right-name"}})

	out, err := Join(left, right, JoinOptions{LeftKey: "id", RightKey: "id", Type: JoinInner})
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	row := out.Rows()[0]
	if row["name"] != "left-name" || row["right_name"] != "right-name" {
		t.Fatalf("expected collision prefixing, got %+v", row)
	}
}

func TestJoinUnknownColumnErrors(t *testing.T) {
	left := FromRecords([]map[string]any{{"id": int64(1)}})
	right := FromRecords([]map[string]any{{"id": int64(1)}})

	if _, err := Join(left, right, JoinOptions{LeftKey: "missing", RightKey: "id"}); err == nil {
		t.Fatal("expected error for unknown left key")
	}
}
