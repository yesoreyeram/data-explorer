package httpclient

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
)

// DigestAuth implements HTTP Digest access authentication (RFC 7616, and
// its RFC 2617 predecessor): unlike every other scheme in this package, it
// cannot simply attach a header up front - the server must first issue a
// challenge (a 401 with a WWW-Authenticate: Digest header containing a
// nonce), which the client then answers with a computed response on a
// retried request. That handshake is implemented as a RoundTripper wrapper
// (see WrapRoundTripper) rather than in Authenticate, which is why DigestAuth
// is the one auth type in this package that must be able to see responses.
type DigestAuth struct {
	Username string
	Password string
}

// Authenticate is a no-op: Digest's credentials are added by the
// RoundTripper returned from WrapRoundTripper, after seeing a 401 challenge.
func (DigestAuth) Authenticate(context.Context, *http.Request) error { return nil }

func (d DigestAuth) WrapRoundTripper(next http.RoundTripper) http.RoundTripper {
	return &digestRoundTripper{next: next, username: d.Username, password: d.Password}
}

type digestRoundTripper struct {
	next               http.RoundTripper
	username, password string
	nonceCount         uint32
}

func (rt *digestRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	resp, err := rt.next.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		return resp, err
	}

	challenge := resp.Header.Get("WWW-Authenticate")
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(challenge)), "digest") {
		return resp, nil // not a digest challenge - nothing we can do
	}
	io.Copy(io.Discard, resp.Body) //nolint:errcheck
	resp.Body.Close()

	params := parseDigestChallenge(challenge)
	authHeader, err := rt.buildAuthorization(params, req.Method, req.URL.RequestURI(), bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("httpclient: digest auth: %w", err)
	}

	retry := req.Clone(req.Context())
	if bodyBytes != nil {
		retry.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}
	retry.Header.Set("Authorization", authHeader)

	return rt.next.RoundTrip(retry)
}

func (rt *digestRoundTripper) buildAuthorization(params map[string]string, method, uri string, body []byte) (string, error) {
	realm, nonce := params["realm"], params["nonce"]
	if nonce == "" {
		return "", fmt.Errorf("challenge missing nonce")
	}

	algorithm := strings.ToUpper(params["algorithm"])
	hashFn := md5Hex
	if strings.HasPrefix(algorithm, "SHA-256") {
		hashFn = sha256Hex
	}

	qop := firstQop(params["qop"])
	cnonce := ""
	if qop != "" {
		cnonce = randomHex(8)
	}
	nc := atomic.AddUint32(&rt.nonceCount, 1)
	ncStr := fmt.Sprintf("%08x", nc)

	ha1 := hashFn(rt.username + ":" + realm + ":" + rt.password)
	var ha2 string
	if qop == "auth-int" {
		ha2 = hashFn(method + ":" + uri + ":" + hashFn(string(body)))
	} else {
		ha2 = hashFn(method + ":" + uri)
	}

	var response string
	if qop == "" {
		response = hashFn(ha1 + ":" + nonce + ":" + ha2)
	} else {
		response = hashFn(ha1 + ":" + nonce + ":" + ncStr + ":" + cnonce + ":" + qop + ":" + ha2)
	}

	var b strings.Builder
	fmt.Fprintf(&b, `Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s"`,
		rt.username, realm, nonce, uri, response)
	if opaque, ok := params["opaque"]; ok {
		fmt.Fprintf(&b, `, opaque="%s"`, opaque)
	}
	if algorithm != "" {
		fmt.Fprintf(&b, `, algorithm=%s`, algorithm)
	}
	if qop != "" {
		fmt.Fprintf(&b, `, qop=%s, nc=%s, cnonce="%s"`, qop, ncStr, cnonce)
	}
	return b.String(), nil
}

// firstQop picks "auth" over "auth-int" when a server offers both
// (comma-separated, e.g. `qop="auth,auth-int"`), since auth-int requires
// hashing the request body and most servers accept plain auth.
func firstQop(raw string) string {
	if raw == "" {
		return ""
	}
	for _, part := range strings.Split(raw, ",") {
		if strings.TrimSpace(part) == "auth" {
			return "auth"
		}
	}
	parts := strings.Split(raw, ",")
	return strings.TrimSpace(parts[0])
}

// parseDigestChallenge parses a `Digest k1="v1", k2=v2, ...` header value
// into a key -> value map, honoring quoted values.
func parseDigestChallenge(header string) map[string]string {
	header = strings.TrimSpace(header)
	header = strings.TrimPrefix(header, "Digest")
	header = strings.TrimPrefix(header, "digest")

	params := map[string]string{}
	for _, field := range splitDigestFields(header) {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		eq := strings.IndexByte(field, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(field[:eq])
		val := strings.TrimSpace(field[eq+1:])
		val = strings.Trim(val, `"`)
		params[key] = val
	}
	return params
}

// splitDigestFields splits on commas that are not inside a quoted string.
func splitDigestFields(s string) []string {
	var fields []string
	var cur strings.Builder
	inQuotes := false
	for _, r := range s {
		switch r {
		case '"':
			inQuotes = !inQuotes
			cur.WriteRune(r)
		case ',':
			if inQuotes {
				cur.WriteRune(r)
			} else {
				fields = append(fields, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		fields = append(fields, cur.String())
	}
	return fields
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func randomHex(n int) string {
	buf := make([]byte, n)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
