package connectors

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

func testCloudWatchLogs(ctx context.Context, awsCfg aws.Config) error {
	client := cloudwatchlogs.NewFromConfig(awsCfg)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := client.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{Limit: aws.Int32(1)})
	return err
}

// executeCloudWatchLogs runs a CloudWatch Logs Insights query - like
// Athena, a start-then-poll API rather than a synchronous call.
func executeCloudWatchLogs(ctx context.Context, awsCfg aws.Config, spec connections.QuerySpec) (*dataframe.Frame, error) {
	if spec.Cloud.Query == "" {
		return nil, fmt.Errorf("cloudwatchLogs requires cloud.query (a Logs Insights query string)")
	}
	if len(spec.Cloud.LogGroupNames) == 0 {
		return nil, fmt.Errorf("cloudwatchLogs requires cloud.logGroupNames")
	}

	end := time.Now()
	if spec.Cloud.EndTime != nil {
		end = *spec.Cloud.EndTime
	}
	start := end.Add(-1 * time.Hour)
	if spec.Cloud.StartTime != nil {
		start = *spec.Cloud.StartTime
	}

	client := cloudwatchlogs.NewFromConfig(awsCfg)
	limit := connections.EffectiveRowLimit(spec.RowLimit)

	startOut, err := client.StartQuery(ctx, &cloudwatchlogs.StartQueryInput{
		LogGroupNames: spec.Cloud.LogGroupNames,
		QueryString:   aws.String(spec.Cloud.Query),
		StartTime:     aws.Int64(start.Unix()),
		EndTime:       aws.Int64(end.Unix()),
		Limit:         aws.Int32(int32(limit)),
	})
	if err != nil {
		return nil, fmt.Errorf("start cloudwatch logs query: %w", err)
	}
	queryID := aws.ToString(startOut.QueryId)

	results, err := pollCloudWatchLogsQuery(ctx, client, queryID)
	if err != nil {
		return nil, err
	}

	frame := dataframe.New(nil)
	truncated := false
	for _, fields := range results {
		if frame.NumRows() >= limit {
			truncated = true
			break
		}
		row := make(map[string]any, len(fields))
		for _, f := range fields {
			row[aws.ToString(f.Field)] = aws.ToString(f.Value)
		}
		frame.AppendRow(row)
	}

	frame.SetMeta(dataframe.Metadata{
		SourceType: "aws:cloudwatchLogs",
		Truncated:  truncated,
		Extra:      map[string]any{"queryId": queryID},
	})
	return frame, nil
}

func pollCloudWatchLogsQuery(ctx context.Context, client *cloudwatchlogs.Client, queryID string) ([][]types.ResultField, error) {
	deadline := time.Now().Add(AsyncQueryMaxWait)
	for {
		out, err := client.GetQueryResults(ctx, &cloudwatchlogs.GetQueryResultsInput{QueryId: aws.String(queryID)})
		if err != nil {
			return nil, fmt.Errorf("get cloudwatch logs query results: %w", err)
		}
		switch out.Status {
		case types.QueryStatusComplete:
			return out.Results, nil
		case types.QueryStatusFailed, types.QueryStatusCancelled, types.QueryStatusTimeout:
			return nil, fmt.Errorf("cloudwatch logs query %s", strings.ToLower(string(out.Status)))
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("cloudwatch logs query did not complete within %s", AsyncQueryMaxWait)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(AsyncQueryPollInterval):
		}
	}
}
