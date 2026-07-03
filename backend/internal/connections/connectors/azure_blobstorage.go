package connectors

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

func blobServiceURL(account string) string {
	return fmt.Sprintf("https://%s.blob.core.windows.net/", account)
}

func testBlobStorage(ctx context.Context, cred azcore.TokenCredential, cfg AzureConfig) error {
	client, err := azblob.NewClient(blobServiceURL(cfg.StorageAccount), cred, nil)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	pager := client.NewListContainersPager(nil)
	_, err = pager.NextPage(ctx)
	return err
}

// executeBlobStorage mirrors executeS3/executeGCS: cloud.key set reads and
// parses one blob (cloud.bucket names the container), otherwise
// cloud.prefix lists blobs as rows. See objectparse.go for the shared
// CSV/JSON/NDJSON parsing.
func executeBlobStorage(ctx context.Context, cred azcore.TokenCredential, cfg AzureConfig, spec connections.QuerySpec) (*dataframe.Frame, error) {
	c := spec.Cloud
	if c.Bucket == "" {
		return nil, fmt.Errorf("blobStorage requires cloud.bucket (the container name)")
	}

	client, err := azblob.NewClient(blobServiceURL(cfg.StorageAccount), cred, nil)
	if err != nil {
		return nil, fmt.Errorf("create client: %w", err)
	}

	if c.Key != "" {
		return blobGetObject(ctx, client, c)
	}
	return blobListObjects(ctx, client, spec)
}

func blobGetObject(ctx context.Context, client *azblob.Client, c *connections.CloudQuerySpec) (*dataframe.Frame, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := client.DownloadStream(ctx, c.Bucket, c.Key, nil)
	if err != nil {
		return nil, fmt.Errorf("download blob: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxObjectBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read blob: %w", err)
	}
	truncated := int64(len(data)) > MaxObjectBytes
	if truncated {
		data = data[:MaxObjectBytes]
	}

	format := c.Format
	if format == "" {
		format = InferObjectFormat(c.Key)
	}
	rows, err := ParseObjectRows(data, format, c.Delimiter)
	if err != nil {
		return nil, err
	}

	frame := dataframe.FromRecords(rows)
	frame.SetMeta(dataframe.Metadata{SourceType: "azure:blobStorage", Truncated: truncated, Extra: map[string]any{"key": c.Key}})
	return frame, nil
}

func blobListObjects(ctx context.Context, client *azblob.Client, spec connections.QuerySpec) (*dataframe.Frame, error) {
	c := spec.Cloud
	limit := connections.EffectiveRowLimit(spec.RowLimit)
	frame := dataframe.New(nil)
	truncated := false

	pager := client.NewListBlobsFlatPager(c.Bucket, &azblob.ListBlobsFlatOptions{Prefix: nonEmptyPtr(c.Prefix)})
outer:
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list blobs: %w", err)
		}
		for _, item := range page.Segment.BlobItems {
			if frame.NumRows() >= limit {
				truncated = true
				break outer
			}
			row := map[string]any{"key": deref(item.Name)}
			if item.Properties != nil {
				if item.Properties.ContentLength != nil {
					row["size"] = *item.Properties.ContentLength
				}
				if item.Properties.LastModified != nil {
					row["lastModified"] = *item.Properties.LastModified
				}
				if item.Properties.ETag != nil {
					row["etag"] = string(*item.Properties.ETag)
				}
			}
			frame.AppendRow(row)
		}
	}

	frame.SetMeta(dataframe.Metadata{SourceType: "azure:blobStorage", Truncated: truncated, Extra: map[string]any{"prefix": c.Prefix}})
	return frame, nil
}
