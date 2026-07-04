package dataframe

import (
	"testing"
	"time"
)

func TestInferType(t *testing.T) {
	cases := []struct {
		v    any
		want Type
	}{
		{nil, TypeNull},
		{"x", TypeString},
		{true, TypeBool},
		{int64(1), TypeInt64},
		{3.14, TypeFloat64},
		{time.Now(), TypeTime},
		{map[string]any{"a": 1}, TypeJSON},
		{[]any{1, 2}, TypeJSON},
	}
	for _, tc := range cases {
		if got := InferType(tc.v); got != tc.want {
			t.Errorf("InferType(%#v) = %s, want %s", tc.v, got, tc.want)
		}
	}
}

func TestUnifyType(t *testing.T) {
	cases := []struct {
		a, b, want Type
	}{
		{TypeInt64, TypeInt64, TypeInt64},
		{TypeInt64, TypeFloat64, TypeFloat64},
		{TypeFloat64, TypeInt64, TypeFloat64},
		{TypeNull, TypeString, TypeString},
		{TypeString, TypeNull, TypeString},
		{TypeString, TypeBool, TypeJSON},
	}
	for _, tc := range cases {
		if got := unifyType(tc.a, tc.b); got != tc.want {
			t.Errorf("unifyType(%s, %s) = %s, want %s", tc.a, tc.b, got, tc.want)
		}
	}
}
