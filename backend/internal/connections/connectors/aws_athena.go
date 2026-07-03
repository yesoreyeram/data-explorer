package connectors

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/athena/types"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

func testAthena(ctx context.Context, awsCfg aws.Config, cfg AWSConfig) error {
	client := athena.NewFromConfig(awsCfg)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := client.ListWorkGroups(ctx, &athena.ListWorkGroupsInput{})
	return err
}

// executeAthena runs a SQL query via Athena's start-then-poll API (there is
// no synchronous "run and wait" call) and converts the result to a Frame.
// EnsureReadOnlySQL applies here too - Athena queries run against tables in
// a Glue/Hive catalog, and DDL/DML through this API is just as much a
// mutation risk as it would be against Postgres/MySQL.
func executeAthena(ctx context.Context, awsCfg aws.Config, cfg AWSConfig, spec connections.QuerySpec) (*dataframe.Frame, error) {
	if spec.Cloud.Query == "" {
		return nil, fmt.Errorf("athena requires cloud.query")
	}
	if err := EnsureReadOnlySQL(spec.Cloud.Query); err != nil {
		return nil, err
	}

	client := athena.NewFromConfig(awsCfg)

	start, err := client.StartQueryExecution(ctx, &athena.StartQueryExecutionInput{
		QueryString:           aws.String(spec.Cloud.Query),
		QueryExecutionContext: &types.QueryExecutionContext{Database: nonEmptyPtr(cfg.AthenaDatabase)},
		WorkGroup:             nonEmptyPtr(cfg.AthenaWorkgroup),
		ResultConfiguration:   athenaResultConfig(cfg.AthenaOutputLocation),
	})
	if err != nil {
		return nil, fmt.Errorf("start athena query: %w", err)
	}
	queryID := aws.ToString(start.QueryExecutionId)

	if err := pollAthenaQuery(ctx, client, queryID); err != nil {
		return nil, err
	}

	limit := connections.EffectiveRowLimit(spec.RowLimit)
	frame := dataframe.New(nil)
	truncated := false

	var nextToken *string
	firstPage := true
	for {
		out, err := client.GetQueryResults(ctx, &athena.GetQueryResultsInput{
			QueryExecutionId: aws.String(queryID),
			NextToken:        nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("get athena query results: %w", err)
		}
		if out.ResultSet == nil {
			break
		}

		var columns []string
		for _, c := range out.ResultSet.ResultSetMetadata.ColumnInfo {
			columns = append(columns, aws.ToString(c.Name))
		}

		rows := out.ResultSet.Rows
		if firstPage && len(rows) > 0 {
			rows = rows[1:] // Athena's first row of a SELECT result is the header row
			firstPage = false
		}

		for _, row := range rows {
			if frame.NumRows() >= limit {
				truncated = true
				break
			}
			r := make(map[string]any, len(columns))
			for i, datum := range row.Data {
				if i >= len(columns) {
					break
				}
				if datum.VarCharValue != nil {
					r[columns[i]] = *datum.VarCharValue
				} else {
					r[columns[i]] = nil
				}
			}
			frame.AppendRow(r)
		}

		if truncated || out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	frame.SetMeta(dataframe.Metadata{
		SourceType: "aws:athena",
		Truncated:  truncated,
		Extra:      map[string]any{"queryExecutionId": queryID},
	})
	return frame, nil
}

func pollAthenaQuery(ctx context.Context, client *athena.Client, queryID string) error {
	deadline := time.Now().Add(AsyncQueryMaxWait)
	for {
		out, err := client.GetQueryExecution(ctx, &athena.GetQueryExecutionInput{QueryExecutionId: aws.String(queryID)})
		if err != nil {
			return fmt.Errorf("get athena query execution: %w", err)
		}
		status := out.QueryExecution.Status
		switch status.State {
		case types.QueryExecutionStateSucceeded:
			return nil
		case types.QueryExecutionStateFailed, types.QueryExecutionStateCancelled:
			return fmt.Errorf("athena query %s: %s", strings.ToLower(string(status.State)), aws.ToString(status.StateChangeReason))
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("athena query did not complete within %s", AsyncQueryMaxWait)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(AsyncQueryPollInterval):
		}
	}
}

func athenaResultConfig(outputLocation string) *types.ResultConfiguration {
	if outputLocation == "" {
		return nil
	}
	return &types.ResultConfiguration{OutputLocation: aws.String(outputLocation)}
}

func nonEmptyPtr(s string) *string {
	if s == "" {
		return nil
	}
	return aws.String(s)
}
