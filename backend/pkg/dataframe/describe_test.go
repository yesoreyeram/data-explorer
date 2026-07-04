package dataframe

import "testing"

func TestDescribeNumericColumn(t *testing.T) {
	f := FromRecords([]map[string]any{
		{"amount": int64(10)},
		{"amount": int64(20)},
		{"amount": nil},
	})

	summaries := f.Describe()
	var amount ColumnSummary
	for _, s := range summaries {
		if s.Name == "amount" {
			amount = s
		}
	}

	if amount.Count != 2 || amount.NullCount != 1 {
		t.Fatalf("expected count=2 nullCount=1, got %+v", amount)
	}
	if amount.Mean == nil || *amount.Mean != 15 {
		t.Fatalf("expected mean=15, got %+v", amount.Mean)
	}
	if amount.Min == nil || *amount.Min != 10 {
		t.Fatalf("expected min=10, got %+v", amount.Min)
	}
}

func TestDescribeStringColumn(t *testing.T) {
	f := FromRecords([]map[string]any{{"name": "ab"}, {"name": "abcd"}})
	summaries := f.Describe()
	if summaries[0].MinLen == nil || *summaries[0].MinLen != 2 {
		t.Fatalf("expected minLen=2, got %+v", summaries[0])
	}
	if summaries[0].MaxLen == nil || *summaries[0].MaxLen != 4 {
		t.Fatalf("expected maxLen=4, got %+v", summaries[0])
	}
}
