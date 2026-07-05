package connectors

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

func testS3(ctx context.Context, awsCfg aws.Config) error {
	client := s3.NewFromConfig(awsCfg)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	_, err := client.ListBuckets(ctx, &s3.ListBucketsInput{MaxBuckets: aws.Int32(1)})
	return err
}

// executeS3 either reads and parses a single object (cloud.key set) or
// lists objects under a prefix as rows (cloud.key empty) - see
// objectparse.go for the shared CSV/JSON/NDJSON parsing this shares with
// the GCS and Azure Blob Storage connectors.
func executeS3(ctx context.Context, awsCfg aws.Config, spec connections.QuerySpec) (*dataframe.Frame, error) {
	c := spec.Cloud
	if c.Bucket == "" {
		return nil, fmt.Errorf("s3 requires cloud.bucket")
	}

	client := s3.NewFromConfig(awsCfg)

	if c.Key != "" {
		return s3GetObject(ctx, client, c, spec.RowLimit)
	}
	return s3ListObjects(ctx, client, spec)
}

func s3GetObject(ctx context.Context, client *s3.Client, c *connections.CloudQuerySpec, rowLimit int) (*dataframe.Frame, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	out, err := client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(c.Bucket), Key: aws.String(c.Key)})
	if err != nil {
		return nil, fmt.Errorf("get s3 object: %w", err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(io.LimitReader(out.Body, MaxObjectBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read s3 object: %w", err)
	}
	truncated := int64(len(data)) > MaxObjectBytes
	if truncated {
		data = data[:MaxObjectBytes]
	}

	format := c.Format
	if format == "" {
		format = InferObjectFormat(c.Key)
	}
	frame, err := ParseObjectFrame(bytes.NewReader(data), format, c.Delimiter, connections.EffectiveRowLimit(rowLimit))
	if err != nil {
		return nil, err
	}
	frame.SetMeta(dataframe.Metadata{SourceType: "aws:s3", Truncated: truncated, Extra: map[string]any{"key": c.Key}})
	return frame, nil
}

func s3ListObjects(ctx context.Context, client *s3.Client, spec connections.QuerySpec) (*dataframe.Frame, error) {
	c := spec.Cloud
	limit := connections.EffectiveRowLimit(spec.RowLimit)
	frame := dataframe.New(nil)
	truncated := false

	var token *string
	for {
		out, err := client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(c.Bucket),
			Prefix:            nonEmptyPtr(c.Prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, fmt.Errorf("list s3 objects: %w", err)
		}

		for _, obj := range out.Contents {
			if frame.NumRows() >= limit {
				truncated = true
				break
			}
			frame.AppendRow(map[string]any{
				"key":          aws.ToString(obj.Key),
				"size":         aws.ToInt64(obj.Size),
				"lastModified": aws.ToTime(obj.LastModified),
				"etag":         aws.ToString(obj.ETag),
			})
		}

		if truncated || !aws.ToBool(out.IsTruncated) {
			break
		}
		token = out.NextContinuationToken
	}

	frame.SetMeta(dataframe.Metadata{SourceType: "aws:s3", Truncated: truncated, Extra: map[string]any{"prefix": c.Prefix}})
	return frame, nil
}
