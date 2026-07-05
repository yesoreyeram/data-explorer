package connectors

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

func testGCS(ctx context.Context, opts []option.ClientOption) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	return client.Close()
}

// executeGCS mirrors executeS3: cloud.key set reads and parses one object,
// otherwise cloud.prefix lists objects as rows. See objectparse.go for the
// shared CSV/JSON/NDJSON parsing.
func executeGCS(ctx context.Context, opts []option.ClientOption, spec connections.QuerySpec) (*dataframe.Frame, error) {
	c := spec.Cloud
	if c.Bucket == "" {
		return nil, fmt.Errorf("gcs requires cloud.bucket")
	}

	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer client.Close()

	bucket := client.Bucket(c.Bucket)
	if c.Key != "" {
		return gcsGetObject(ctx, bucket, c, spec.RowLimit)
	}
	return gcsListObjects(ctx, bucket, spec)
}

func gcsGetObject(ctx context.Context, bucket *storage.BucketHandle, c *connections.CloudQuerySpec, rowLimit int) (*dataframe.Frame, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	reader, err := bucket.Object(c.Key).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("read gcs object: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, MaxObjectBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read gcs object: %w", err)
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
	frame.SetMeta(dataframe.Metadata{SourceType: "gcp:gcs", Truncated: truncated, Extra: map[string]any{"key": c.Key}})
	return frame, nil
}

func gcsListObjects(ctx context.Context, bucket *storage.BucketHandle, spec connections.QuerySpec) (*dataframe.Frame, error) {
	c := spec.Cloud
	limit := connections.EffectiveRowLimit(spec.RowLimit)
	frame := dataframe.New(nil)
	truncated := false

	it := bucket.Objects(ctx, &storage.Query{Prefix: c.Prefix})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("list gcs objects: %w", err)
		}
		if frame.NumRows() >= limit {
			truncated = true
			break
		}
		frame.AppendRow(map[string]any{
			"key":          attrs.Name,
			"size":         attrs.Size,
			"lastModified": attrs.Updated,
			"etag":         attrs.Etag,
		})
	}

	frame.SetMeta(dataframe.Metadata{SourceType: "gcp:gcs", Truncated: truncated, Extra: map[string]any{"prefix": c.Prefix}})
	return frame, nil
}
