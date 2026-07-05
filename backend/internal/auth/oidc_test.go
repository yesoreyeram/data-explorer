package auth

import (
	"context"
	"errors"
	"testing"
)

func TestOIDCManagerNilAndEmpty(t *testing.T) {
	var m *OIDCManager
	if m.Enabled() {
		t.Fatal("nil manager should not be enabled")
	}
	if m.Providers() != nil {
		t.Fatal("nil manager should list no providers")
	}
	if _, err := m.AuthCodeURL("x", "s", "v"); !errors.Is(err, ErrOIDCProviderUnknown) {
		t.Fatalf("AuthCodeURL on nil manager = %v, want ErrOIDCProviderUnknown", err)
	}
}

func TestNewOIDCManagerValidation(t *testing.T) {
	// Missing required fields must fail before any network call.
	_, err := NewOIDCManager(context.Background(), []OIDCProviderConfig{{Name: "google"}})
	if err == nil {
		t.Fatal("expected validation error for incomplete provider config")
	}
}
