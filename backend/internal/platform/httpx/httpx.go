// Package httpx holds small helpers shared by every HTTP handler, so
// response shape (JSON envelopes, error codes) is consistent across the
// entire API surface without each handler reinventing it.
package httpx

import (
	"encoding/json"
	"io"
	"net/http"
)

type ErrorBody struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	var body ErrorBody
	body.Error.Code = code
	body.Error.Message = message
	WriteJSON(w, status, body)
}

// DecodeJSON reads and decodes a JSON body with a size cap (1MB) to prevent
// a malicious or buggy client from exhausting server memory on one request.
func DecodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func ClientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return fwd
	}
	return r.RemoteAddr
}

func Drain(body io.ReadCloser) {
	if body != nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(body, 1<<20))
		_ = body.Close()
	}
}
