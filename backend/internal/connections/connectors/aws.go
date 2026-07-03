// Package connectors: AWS support. One connection type ("aws") covers four
// services, selected by AWSConfig.Service, because they share one thing
// that matters more than their individual APIs: how a Data Explorer
// connection authenticates to AWS. That's centralized here in
// awsConfig/credentials so each service file (aws_athena.go,
// aws_cloudwatchlogs.go, aws_dynamodb.go, aws_s3.go) only has to implement
// its own query semantics.
package connectors

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/pkg/dataframe"
)

// AWSConfig is the non-secret configuration for an "aws" connection.
// Credentials (secret keys "accessKeyId"/"secretAccessKey"/"sessionToken")
// are optional: when omitted, the connector falls back to the AWS SDK's
// standard default credential chain (environment variables, shared config
// file, or - the common case in a real deployment - an IAM role attached to
// the EC2 instance/ECS task/EKS pod running this server). That fallback is
// what lets an operator run Data Explorer inside AWS with zero long-lived
// credentials stored in this database at all.
type AWSConfig struct {
	Region string `json:"region"`
	// Service selects which AWS service this connection queries:
	// "athena" | "cloudwatchLogs" | "dynamodb" | "s3".
	Service string `json:"service"`

	// Athena
	AthenaDatabase       string `json:"athenaDatabase,omitempty"`
	AthenaWorkgroup      string `json:"athenaWorkgroup,omitempty"`
	AthenaOutputLocation string `json:"athenaOutputLocation,omitempty"` // s3://bucket/prefix
}

type AWS struct{}

func NewAWS() *AWS { return &AWS{} }

func (a *AWS) parseConfig(cfgJSON json.RawMessage) (AWSConfig, error) {
	var cfg AWSConfig
	if err := json.Unmarshal(cfgJSON, &cfg); err != nil {
		return AWSConfig{}, fmt.Errorf("invalid aws config: %w", err)
	}
	if cfg.Region == "" {
		return AWSConfig{}, fmt.Errorf("region is required")
	}
	switch cfg.Service {
	case "athena", "cloudwatchLogs", "dynamodb", "s3":
	default:
		return AWSConfig{}, fmt.Errorf("unsupported aws service %q", cfg.Service)
	}
	return cfg, nil
}

// awsConfig builds an aws.Config for the given connection: static
// credentials if the secret supplies them, otherwise the SDK's own default
// credential chain.
func awsConfig(ctx context.Context, cfg AWSConfig, secret map[string]string) (aws.Config, error) {
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(cfg.Region)}

	if accessKey := secret["accessKeyId"]; accessKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKey, secret["secretAccessKey"], secret["sessionToken"],
		)))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("load aws credentials: %w", err)
	}
	return awsCfg, nil
}

func (a *AWS) Test(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string) error {
	cfg, err := a.parseConfig(cfgJSON)
	if err != nil {
		return err
	}
	awsCfg, err := awsConfig(ctx, cfg, secret)
	if err != nil {
		return err
	}

	switch cfg.Service {
	case "athena":
		return testAthena(ctx, awsCfg, cfg)
	case "cloudwatchLogs":
		return testCloudWatchLogs(ctx, awsCfg)
	case "dynamodb":
		return testDynamoDB(ctx, awsCfg)
	case "s3":
		return testS3(ctx, awsCfg)
	default:
		return connections.ErrUnsupportedType
	}
}

func (a *AWS) Execute(ctx context.Context, cfgJSON json.RawMessage, secret map[string]string, spec connections.QuerySpec) (*dataframe.Frame, error) {
	cfg, err := a.parseConfig(cfgJSON)
	if err != nil {
		return nil, err
	}
	if spec.Cloud == nil {
		return nil, fmt.Errorf("this connection requires a cloud query spec")
	}
	awsCfg, err := awsConfig(ctx, cfg, secret)
	if err != nil {
		return nil, err
	}

	switch cfg.Service {
	case "athena":
		return executeAthena(ctx, awsCfg, cfg, spec)
	case "cloudwatchLogs":
		return executeCloudWatchLogs(ctx, awsCfg, spec)
	case "dynamodb":
		return executeDynamoDB(ctx, awsCfg, spec)
	case "s3":
		return executeS3(ctx, awsCfg, spec)
	default:
		return nil, connections.ErrUnsupportedType
	}
}
