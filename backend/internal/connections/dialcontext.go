package connections

import (
	"context"
	"net"
)

// DialFunc is the dialer signature shared by http.Transport.DialContext,
// pgconn.Config.DialFunc, and mysql.RegisterDialContext.
type DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error)

// dialCtxKey carries a per-call dialer override. It lives in this package
// (rather than connectors) so the Service can set it without importing
// connectors, which would be an import cycle.
type dialCtxKey struct{}

// WithDialContext overrides, for this request only, the dialer connectors use.
// The Service applies it on the ad-hoc query path so that path can enforce a
// stricter egress policy than saved connections. Connectors fall back to their
// construction-time dialer when the context carries no override.
func WithDialContext(ctx context.Context, dc DialFunc) context.Context {
	if dc == nil {
		return ctx
	}
	return context.WithValue(ctx, dialCtxKey{}, dc)
}

// DialContextFrom returns the per-call dialer override, or nil if none.
func DialContextFrom(ctx context.Context) DialFunc {
	if dc, ok := ctx.Value(dialCtxKey{}).(DialFunc); ok {
		return dc
	}
	return nil
}
