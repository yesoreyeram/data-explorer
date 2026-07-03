package connectors

import (
	"encoding/json"
	"testing"
)

func TestAWSParseConfigRequiresRegion(t *testing.T) {
	a := NewAWS()
	cfgJSON, _ := json.Marshal(AWSConfig{Service: "s3"})
	if _, err := a.parseConfig(cfgJSON); err == nil {
		t.Fatal("expected an error when region is missing")
	}
}

func TestAWSParseConfigRejectsUnknownService(t *testing.T) {
	a := NewAWS()
	cfgJSON, _ := json.Marshal(AWSConfig{Region: "us-east-1", Service: "redshift"})
	if _, err := a.parseConfig(cfgJSON); err == nil {
		t.Fatal("expected an error for an unsupported service")
	}
}

func TestAWSParseConfigAcceptsEachKnownService(t *testing.T) {
	a := NewAWS()
	for _, svc := range []string{"athena", "cloudwatchLogs", "dynamodb", "s3"} {
		cfgJSON, _ := json.Marshal(AWSConfig{Region: "us-east-1", Service: svc})
		if _, err := a.parseConfig(cfgJSON); err != nil {
			t.Errorf("service %q: unexpected error: %v", svc, err)
		}
	}
}
