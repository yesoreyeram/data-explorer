package httpclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// bufferBody reads and restores a request's body, returning the bytes so a
// paginator can build subsequent requests with a (possibly mutated) copy of
// the same body without re-reading an already-consumed reader.
func bufferBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body = io.NopCloser(bytes.NewReader(data))
	return data, nil
}

// cloneWithQuery clones template and applies mutate to its query string.
func cloneWithQuery(template *http.Request, bodyBytes []byte, mutate func(q url.Values)) *http.Request {
	req := template.Clone(template.Context())
	q := req.URL.Query()
	mutate(q)
	req.URL.RawQuery = q.Encode()
	if bodyBytes != nil {
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}
	return req
}

// cloneWithHeader clones template and sets a header on it.
func cloneWithHeader(template *http.Request, bodyBytes []byte, name, value string) *http.Request {
	req := template.Clone(template.Context())
	req.Header.Set(name, value)
	if bodyBytes != nil {
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}
	return req
}

// cloneWithJSONBody clones template, decodes bodyBytes as a JSON object,
// applies mutate, and re-encodes it as the clone's new body.
func cloneWithJSONBody(template *http.Request, bodyBytes []byte, mutate func(body map[string]any)) (*http.Request, error) {
	body := map[string]any{}
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &body); err != nil {
			return nil, fmt.Errorf("pagination: request body is not a JSON object: %w", err)
		}
	}
	mutate(body)

	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req := template.Clone(template.Context())
	req.Body = io.NopCloser(bytes.NewReader(encoded))
	req.ContentLength = int64(len(encoded))
	return req, nil
}
