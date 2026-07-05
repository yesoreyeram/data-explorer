// Package connectors: Azure support. Mirrors aws.go/gcp.go's shape - one
// connection type ("azure") covering multiple services, selected by
// AzureConfig.Service, with authentication centralized here so each
// service file only implements its own query semantics.
package connectors

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

// AzureConfig is the non-secret configuration for an "azure" connection.
// Credentials are optional: with TenantID/ClientID set and either a
// "clientSecret" or a "clientCertificate" (PEM or PKCS12, optionally paired
// with "clientCertificatePassword") in the connection's secret, the
// connector authenticates as that service principal; otherwise it falls
// back to DefaultAzureCredential, which tries (in order) environment
// variables, workload identity, managed identity, and the Azure CLI's
// logged-in session - so a server running inside Azure (e.g. AKS with
// workload identity, or a VM with a managed identity) never needs a
// long-lived secret stored here at all.
type AzureConfig struct {
	// Service selects which Azure service this connection queries:
	// "logAnalytics" | "blobStorage".
	Service  string `json:"service"`
	TenantID string `json:"tenantId,omitempty"`
	ClientID string `json:"clientId,omitempty"`

	// Log Analytics
	WorkspaceID string `json:"workspaceId,omitempty"`

	// Blob Storage
	StorageAccount string `json:"storageAccount,omitempty"`
}

type Azure struct{ opts Options }

func NewAzure(opts Options) *Azure { return &Azure{opts: opts} }

// TODO(egress): route the Azure SDK's HTTP through the egress guard by setting
// azcore.ClientOptions{Transport: guarded} on both azureCredential and each
// service client (azure_blobstorage.go, azure_loganalytics.go). Azure service
// endpoints are fixed provider hosts - the user controls
// workspace/account/query, not the host - so this is defense-in-depth against
// ambient-credential (managed identity / IMDS) resolution, not the
// user-exploitable hole the HTTP/SQL connectors were.

func (a *Azure) parseConfig(cfgJSON json.RawMessage) (AzureConfig, error) {
	var cfg AzureConfig
	if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
		return AzureConfig{}, fmt.Errorf("invalid azure config: %w", err)
	}
	switch cfg.Service {
	case "logAnalytics":
		if cfg.WorkspaceID == "" {
			return AzureConfig{}, fmt.Errorf("logAnalytics requires workspaceId")
		}
	case "blobStorage":
		if cfg.StorageAccount == "" {
			return AzureConfig{}, fmt.Errorf("blobStorage requires storageAccount")
		}
	default:
		return AzureConfig{}, fmt.Errorf("unsupported azure service %q", cfg.Service)
	}
	return cfg, nil
}

func azureCredential(cfg AzureConfig, secret map[string]string) (azcore.TokenCredential, error) {
	if cfg.TenantID != "" && cfg.ClientID != "" {
		if certData := secret["clientCertificate"]; certData != "" {
			certs, key, err := azidentity.ParseCertificates([]byte(certData), []byte(secret["clientCertificatePassword"]))
			if err != nil {
				return nil, fmt.Errorf("parse client certificate: %w", err)
			}
			return azidentity.NewClientCertificateCredential(cfg.TenantID, cfg.ClientID, certs, key, nil)
		}
		if secret["clientSecret"] != "" {
			return azidentity.NewClientSecretCredential(cfg.TenantID, cfg.ClientID, secret["clientSecret"], nil)
		}
	}
	return azidentity.NewDefaultAzureCredential(nil)
}

func (a *Azure) Test(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string) error {
	cfg, err := a.parseConfig(cfgJSON)
	if err != nil {
		return err
	}
	cred, err := azureCredential(cfg, secret)
	if err != nil {
		return fmt.Errorf("configure azure credentials: %w", err)
	}

	switch cfg.Service {
	case "logAnalytics":
		return testLogAnalytics(ctx, cred, cfg)
	case "blobStorage":
		return testBlobStorage(ctx, cred, cfg)
	default:
		return connections.ErrUnsupportedType
	}
}

func (a *Azure) Execute(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string, spec connections.QuerySpec) (*dataframe.Frame, error) {
	cfg, err := a.parseConfig(cfgJSON)
	if err != nil {
		return nil, err
	}
	if spec.Cloud == nil {
		return nil, fmt.Errorf("this connection requires a cloud query spec")
	}
	cred, err := azureCredential(cfg, secret)
	if err != nil {
		return nil, fmt.Errorf("configure azure credentials: %w", err)
	}

	switch cfg.Service {
	case "logAnalytics":
		return executeLogAnalytics(ctx, cred, cfg, spec)
	case "blobStorage":
		return executeBlobStorage(ctx, cred, cfg, spec)
	default:
		return nil, connections.ErrUnsupportedType
	}
}
