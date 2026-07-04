package connectors

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

func testDynamoDB(ctx context.Context, awsCfg aws.Config) error {
	client := dynamodb.NewFromConfig(awsCfg)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := client.ListTables(ctx, &dynamodb.ListTablesInput{Limit: aws.Int32(1)})
	return err
}

// executeDynamoDB runs a Query (key-condition lookup) or Scan (full-table
// read), paginating internally via LastEvaluatedKey until either the
// result is exhausted or the row limit is hit.
func executeDynamoDB(ctx context.Context, awsCfg aws.Config, spec connections.QuerySpec) (*dataframe.Frame, error) {
	c := spec.Cloud
	if c.TableName == "" {
		return nil, fmt.Errorf("dynamodb requires cloud.tableName")
	}
	if !c.Scan && c.KeyConditionExpression == "" {
		return nil, fmt.Errorf("dynamodb query requires cloud.keyConditionExpression (or set cloud.scan=true for a full scan)")
	}

	attrValues, err := attributevalue.MarshalMap(c.ExpressionAttributeValues)
	if err != nil {
		return nil, fmt.Errorf("marshal expressionAttributeValues: %w", err)
	}

	client := dynamodb.NewFromConfig(awsCfg)
	limit := connections.EffectiveRowLimit(spec.RowLimit)
	frame := dataframe.New(nil)
	truncated := false

	var lastKey map[string]ddbtypes.AttributeValue
	for {
		var items []map[string]ddbtypes.AttributeValue
		var nextKey map[string]ddbtypes.AttributeValue

		if c.Scan {
			out, err := client.Scan(ctx, &dynamodb.ScanInput{
				TableName:                 aws.String(c.TableName),
				IndexName:                 nonEmptyPtr(c.IndexName),
				FilterExpression:          nonEmptyPtr(c.FilterExpression),
				ExpressionAttributeNames:  nonEmptyMapPtr(c.ExpressionAttributeNames),
				ExpressionAttributeValues: attrValues,
				ExclusiveStartKey:         lastKey,
			})
			if err != nil {
				return nil, fmt.Errorf("dynamodb scan: %w", err)
			}
			items, nextKey = out.Items, out.LastEvaluatedKey
		} else {
			out, err := client.Query(ctx, &dynamodb.QueryInput{
				TableName:                 aws.String(c.TableName),
				IndexName:                 nonEmptyPtr(c.IndexName),
				KeyConditionExpression:    aws.String(c.KeyConditionExpression),
				FilterExpression:          nonEmptyPtr(c.FilterExpression),
				ExpressionAttributeNames:  nonEmptyMapPtr(c.ExpressionAttributeNames),
				ExpressionAttributeValues: attrValues,
				ExclusiveStartKey:         lastKey,
			})
			if err != nil {
				return nil, fmt.Errorf("dynamodb query: %w", err)
			}
			items, nextKey = out.Items, out.LastEvaluatedKey
		}

		for _, item := range items {
			if frame.NumRows() >= limit {
				truncated = true
				break
			}
			var row map[string]any
			if err := attributevalue.UnmarshalMap(item, &row); err != nil {
				return nil, fmt.Errorf("unmarshal dynamodb item: %w", err)
			}
			frame.AppendRow(row)
		}

		if truncated || len(nextKey) == 0 {
			break
		}
		lastKey = nextKey
	}

	frame.SetMeta(dataframe.Metadata{SourceType: "aws:dynamodb", Truncated: truncated})
	return frame, nil
}

func nonEmptyMapPtr(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	return m
}
