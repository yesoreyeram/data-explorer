package httpx

import (
	"bytes"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSONRejectsOversizedBody(t *testing.T) {
	huge := strings.Repeat("a", MaxRequestBodyBytes+1)
	body := `{"field":"` + huge + `"}`

	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(body)))
	var v map[string]string
	err := DecodeJSON(req, &v)
	if !errors.Is(err, ErrPayloadTooLarge) {
		t.Fatalf("expected ErrPayloadTooLarge, got %v", err)
	}
}

func TestDecodeJSONAcceptsNormalBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/", bytes.NewReader([]byte(`{"field":"value"}`)))
	var v struct {
		Field string `json:"field"`
	}
	if err := DecodeJSON(req, &v); err != nil {
		t.Fatalf("DecodeJSON: %v", err)
	}
	if v.Field != "value" {
		t.Fatalf("expected field=value, got %q", v.Field)
	}
}
