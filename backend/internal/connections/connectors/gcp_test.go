package connectors

import (
	"context"
	"encoding/json"
	"testing"
)

func TestGCPParseConfigRequiresProjectID(t *testing.T) {
	g := NewGCP(Options{})
	cfgJSON, _ := json.Marshal(GCPConfig{Service: "bigquery"})
	if _, err := g.parseConfig(cfgJSON); err == nil {
		t.Fatal("expected an error when projectId is missing")
	}
}

func TestGCPParseConfigRejectsUnknownService(t *testing.T) {
	g := NewGCP(Options{})
	cfgJSON, _ := json.Marshal(GCPConfig{ProjectID: "p1", Service: "spanner"})
	if _, err := g.parseConfig(cfgJSON); err == nil {
		t.Fatal("expected an error for an unsupported service")
	}
}

func TestGCPParseConfigAcceptsEachKnownService(t *testing.T) {
	g := NewGCP(Options{})
	for _, svc := range []string{"bigquery", "gcs"} {
		cfgJSON, _ := json.Marshal(GCPConfig{ProjectID: "p1", Service: svc})
		if _, err := g.parseConfig(cfgJSON); err != nil {
			t.Errorf("service %q: unexpected error: %v", svc, err)
		}
	}
}

func TestGCPClientOptionsUsesServiceAccountKeyWhenPresent(t *testing.T) {
	opts, err := gcpClientOptions(context.Background(), GCPConfig{}, map[string]string{"serviceAccountKeyJson": `{"type":"service_account"}`})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts) != 1 {
		t.Fatalf("expected 1 client option when a key is present, got %d", len(opts))
	}
}

func TestGCPClientOptionsEmptyWithoutKey(t *testing.T) {
	opts, err := gcpClientOptions(context.Background(), GCPConfig{}, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts) != 0 {
		t.Fatalf("expected no client options without a key (ADC fallback), got %d", len(opts))
	}
}
