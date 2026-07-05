package connectors

import (
	"context"
	"fmt"

	"github.com/yesoreyeram/data-explorer/backend/internal/connections"
	"github.com/yesoreyeram/data-explorer/backend/pkg/httpclient"
)

// Options configures how connectors dial out. The zero value reproduces the
// pre-hardening behavior exactly (default dialer, no egress guard, default
// caps), so passing Options{} is always safe.
type Options struct {
	// DialContext, when set, is the egress-guarded dialer every outbound
	// connection uses - HTTP(S), Postgres/MySQL TCP, cloud SDK HTTP, and the
	// OAuth2/workload-identity token endpoints. Nil uses the default dialer.
	DialContext httpclient.DialFunc
	// MaxResponseBytes caps HTTP response bodies (REST/GraphQL). 0 uses the
	// httpclient default (25MB).
	MaxResponseBytes int64
	// UserAgent, when set, identifies outbound HTTP requests.
	UserAgent string
	// StrictHeaders rejects reserved/hop-by-hop user-supplied request headers
	// on the REST connector.
	StrictHeaders bool
}

// dial resolves the effective dialer: a per-call override (set by the Service
// on the ad-hoc path via connections.WithDialContext) wins over the
// connector's construction-time Options dialer.
func (o Options) dial(ctx context.Context) httpclient.DialFunc {
	if dc := connections.DialContextFrom(ctx); dc != nil {
		return dc
	}
	return o.DialContext
}

// RegisterAll constructs and registers the named connector types with opts.
// Unknown types are an error so a misconfiguration fails fast at startup.
func RegisterAll(reg *connections.Registry, types []string, opts Options) error {
	for _, t := range types {
		var c connections.Connector
		switch t {
		case "postgres":
			c = NewPostgres(opts)
		case "mysql":
			c = NewMySQL(opts)
		case "rest":
			c = NewREST(opts)
		case "graphql":
			c = NewGraphQL(opts)
		case "aws":
			c = NewAWS(opts)
		case "gcp":
			c = NewGCP(opts)
		case "azure":
			c = NewAzure(opts)
		default:
			return fmt.Errorf("connectors: unknown type %q", t)
		}
		reg.Register(t, c)
	}
	return nil
}
