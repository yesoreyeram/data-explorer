package dataframe

import "testing"

func TestGroupByAggregations(t *testing.T) {
	f := FromRecords([]map[string]any{
		{"region": "east", "amount": int64(100)},
		{"region": "east", "amount": int64(50)},
		{"region": "west", "amount": int64(30)},
	})

	out, err := f.GroupBy([]string{"region"}, []Agg{
		{Field: "amount", Op: AggSum, As: "total"},
		{Field: "amount", Op: AggCount, As: "n"},
	})
	if err != nil {
		t.Fatalf("GroupBy: %v", err)
	}
	if out.NumRows() != 2 {
		t.Fatalf("expected 2 groups, got %d", out.NumRows())
	}

	byRegion := map[string]map[string]any{}
	for _, row := range out.Rows() {
		byRegion[row["region"].(string)] = row
	}
	if byRegion["east"]["total"] != float64(150) {
		t.Fatalf("expected east total=150, got %+v", byRegion["east"])
	}
	if byRegion["west"]["n"] != int64(1) {
		t.Fatalf("expected west n=1, got %+v", byRegion["west"])
	}
}

func TestGroupByUnknownKeyErrors(t *testing.T) {
	f := FromRecords([]map[string]any{{"a": int64(1)}})
	if _, err := f.GroupBy([]string{"missing"}, []Agg{{Field: "a", Op: AggSum}}); err == nil {
		t.Fatal("expected error for unknown group key")
	}
}
