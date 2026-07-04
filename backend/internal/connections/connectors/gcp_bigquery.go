package connectors

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

func testBigQuery(ctx context.Context, cfg GCPConfig, opts []option.ClientOption) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := bigquery.NewClient(ctx, cfg.ProjectID, opts...)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	// A trivial constant query is BigQuery's equivalent of "ping": it
	// exercises auth and API reachability without touching any dataset.
	it, err := client.Query("SELECT 1").Read(ctx)
	if err != nil {
		return err
	}
	var row []bigquery.Value
	if err := it.Next(&row); err != nil && err != iterator.Done {
		return err
	}
	return nil
}

// executeBigQuery runs a SQL query - unlike Athena/CloudWatch Logs
// Insights, the client library's Query.Read call already blocks until the
// job completes, so there's no separate poll loop needed here.
func executeBigQuery(ctx context.Context, cfg GCPConfig, opts []option.ClientOption, spec connections.QuerySpec) (*dataframe.Frame, error) {
	if spec.Cloud.Query == "" {
		return nil, fmt.Errorf("bigquery requires cloud.query")
	}
	if err := EnsureReadOnlySQL(spec.Cloud.Query); err != nil {
		return nil, err
	}

	client, err := bigquery.NewClient(ctx, cfg.ProjectID, opts...)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	it, err := client.Query(spec.Cloud.Query).Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("run bigquery query: %w", err)
	}

	var columns []string
	for _, f := range it.Schema {
		columns = append(columns, f.Name)
	}

	limit := connections.EffectiveRowLimit(spec.RowLimit)
	frame := dataframe.New(nil)
	truncated := false

	for {
		var values []bigquery.Value
		err := it.Next(&values)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read bigquery results: %w", err)
		}
		if frame.NumRows() >= limit {
			truncated = true
			break
		}

		row := make(map[string]any, len(columns))
		for i, v := range values {
			if i < len(columns) {
				row[columns[i]] = v
			}
		}
		frame.AppendRow(row)
	}

	frame.SetMeta(dataframe.Metadata{SourceType: "gcp:bigquery", Truncated: truncated})
	return frame, nil
}
