package connectors

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/monitor/azquery"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

func testLogAnalytics(ctx context.Context, cred azcore.TokenCredential, cfg AzureConfig) error {
	client, err := azquery.NewLogsClient(cred, nil)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err = client.QueryWorkspace(ctx, cfg.WorkspaceID, azquery.Body{
		Query:    to.Ptr("print 1"),
		Timespan: to.Ptr(azquery.NewTimeInterval(time.Now().Add(-5*time.Minute), time.Now())),
	}, nil)
	return err
}

// executeLogAnalytics runs a KQL query against a Log Analytics workspace -
// Azure's equivalent of CloudWatch Logs Insights/BigQuery for log/telemetry
// data. Unlike Athena/CloudWatch Logs, QueryWorkspace is synchronous.
func executeLogAnalytics(ctx context.Context, cred azcore.TokenCredential, cfg AzureConfig, spec connections.QuerySpec) (*dataframe.Frame, error) {
	if spec.Cloud.Query == "" {
		return nil, fmt.Errorf("logAnalytics requires cloud.query (a KQL query)")
	}

	client, err := azquery.NewLogsClient(cred, nil)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	end := time.Now()
	if spec.Cloud.EndTime != nil {
		end = *spec.Cloud.EndTime
	}
	start := end.Add(-1 * time.Hour)
	if spec.Cloud.StartTime != nil {
		start = *spec.Cloud.StartTime
	}

	ctx, cancel := context.WithTimeout(ctx, AsyncQueryMaxWait)
	defer cancel()

	resp, err := client.QueryWorkspace(ctx, cfg.WorkspaceID, azquery.Body{
		Query:    to.Ptr(spec.Cloud.Query),
		Timespan: to.Ptr(azquery.NewTimeInterval(start, end)),
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("query log analytics: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("log analytics query error: %w", resp.Error)
	}

	limit := connections.EffectiveRowLimit(spec.RowLimit)
	frame := dataframe.New(nil)
	truncated := false

outer:
	for _, table := range resp.Tables {
		var columns []string
		for _, c := range table.Columns {
			columns = append(columns, deref(c.Name))
		}
		for _, row := range table.Rows {
			if frame.NumRows() >= limit {
				truncated = true
				break outer
			}
			r := make(map[string]any, len(columns))
			for i, v := range row {
				if i < len(columns) {
					r[columns[i]] = v
				}
			}
			frame.AppendRow(r)
		}
	}

	frame.SetMeta(dataframe.Metadata{SourceType: "azure:logAnalytics", Truncated: truncated})
	return frame, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
