package connectors

import "time"

// Guardrails shared by every cloud provider connector (AWS/GCP/Azure).

// MaxObjectBytes caps how much of a single object storage file
// (S3/GCS/Blob Storage) is downloaded and parsed, protecting the server
// from a multi-gigabyte file being read into memory whole just because a
// user picked the wrong key.
const MaxObjectBytes = 50 * 1024 * 1024 // 50MB

// Async query services (Athena, CloudWatch Logs Insights) are
// start-then-poll APIs with no native "wait" call; these bound that polling
// loop so a query that never finishes (or an API that stops responding)
// can't hang a request indefinitely or spin forever.
const (
	AsyncQueryPollInterval = 500 * time.Millisecond
	AsyncQueryMaxWait      = 55 * time.Second
)
