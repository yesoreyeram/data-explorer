package connectors

import (
	"encoding/json"
	"testing"
)

func TestAzureParseConfigLogAnalyticsRequiresWorkspaceID(t *testing.T) {
	a := NewAzure(Options{})
	cfgJSON, _ := json.Marshal(AzureConfig{Service: "logAnalytics"})
	if _, err := a.parseConfig(cfgJSON); err == nil {
		t.Fatal("expected an error when workspaceId is missing")
	}
}

func TestAzureParseConfigBlobStorageRequiresStorageAccount(t *testing.T) {
	a := NewAzure(Options{})
	cfgJSON, _ := json.Marshal(AzureConfig{Service: "blobStorage"})
	if _, err := a.parseConfig(cfgJSON); err == nil {
		t.Fatal("expected an error when storageAccount is missing")
	}
}

func TestAzureParseConfigRejectsUnknownService(t *testing.T) {
	a := NewAzure(Options{})
	cfgJSON, _ := json.Marshal(AzureConfig{Service: "cosmosdb"})
	if _, err := a.parseConfig(cfgJSON); err == nil {
		t.Fatal("expected an error for an unsupported service")
	}
}

func TestAzureParseConfigAcceptsEachKnownService(t *testing.T) {
	a := NewAzure(Options{})
	cases := []AzureConfig{
		{Service: "logAnalytics", WorkspaceID: "ws-1"},
		{Service: "blobStorage", StorageAccount: "acct1"},
	}
	for _, cfg := range cases {
		cfgJSON, _ := json.Marshal(cfg)
		if _, err := a.parseConfig(cfgJSON); err != nil {
			t.Errorf("service %q: unexpected error: %v", cfg.Service, err)
		}
	}
}

func TestBlobServiceURL(t *testing.T) {
	if got := blobServiceURL("myaccount"); got != "https://myaccount.blob.core.windows.net/" {
		t.Fatalf("unexpected blob service URL: %q", got)
	}
}
