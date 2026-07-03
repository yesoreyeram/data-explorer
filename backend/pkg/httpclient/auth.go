package httpclient

import (
	"context"
	"net/http"
)

// Authenticator adds credentials to an outgoing request. Implementations
// must be safe for concurrent use, since a Client may serve many concurrent
// requests. Most schemes (Basic, Bearer, API key, JWT, OAuth2) only need to
// set a header or query parameter and are pure "mutate before send"
// authenticators.
type Authenticator interface {
	Authenticate(ctx context.Context, req *http.Request) error
}

// RoundTripperAuthenticator is implemented by auth schemes that need to
// inspect a response before they can authenticate - currently only Digest,
// whose handshake is: send unauthenticated, read the 401 challenge, resend
// with a computed response. Client.New wraps the transport with this when
// the configured Authenticator provides it.
type RoundTripperAuthenticator interface {
	Authenticator
	WrapRoundTripper(next http.RoundTripper) http.RoundTripper
}

// NoAuth sends requests unmodified. It is the zero value's effective
// behavior (a nil Authenticator), provided explicitly for readability at
// call sites that want to be unambiguous about the choice.
type NoAuth struct{}

func (NoAuth) Authenticate(context.Context, *http.Request) error { return nil }
