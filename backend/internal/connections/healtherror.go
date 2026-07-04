package connections

import (
	"context"
	"errors"
	"net"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/aws/smithy-go"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/api/googleapi"
)

// ErrorCode is a small, fixed vocabulary every connector's error - whatever
// SQL driver, HTTP client, or cloud SDK produced it - gets classified into.
// It's what the UI keys a badge/icon off of; it is not meant to be
// exhaustive, just enough to separate "your credentials are wrong" from
// "the network is unreachable" from "you don't have permission", since
// those need genuinely different next steps.
type ErrorCode string

const (
	ErrCodeTimeout            ErrorCode = "timeout"
	ErrCodeNetworkUnreachable ErrorCode = "network_unreachable"
	ErrCodeAuthFailed         ErrorCode = "auth_failed"
	ErrCodePermissionDenied   ErrorCode = "permission_denied"
	ErrCodeNotFound           ErrorCode = "not_found"
	ErrCodeRateLimited        ErrorCode = "rate_limited"
	ErrCodeInvalidConfig      ErrorCode = "invalid_config"
	ErrCodeUnknown            ErrorCode = "unknown"
)

// HealthError is the structured, user-facing shape every connector error is
// classified into (see Classify) before it's persisted on a Connection or
// returned from the API. Message is written for whoever is looking at a
// failed connection in the UI, not a Go developer; Remediation is the next
// concrete thing to try. The original error is preserved (Unwrap) so logs
// and errors.Is/As checks upstream keep working.
type HealthError struct {
	Code        ErrorCode
	Message     string
	Remediation string
	cause       error
}

func (e *HealthError) Error() string {
	if e.cause != nil {
		return e.Message + ": " + e.cause.Error()
	}
	return e.Message
}

func (e *HealthError) Unwrap() error { return e.cause }

// Detail returns the underlying technical error message, if any - surfaced
// in the UI as a collapsed/secondary line under the friendly Message, for
// whoever needs to go dig further (or file a support ticket).
func (e *HealthError) Detail() string {
	if e.cause == nil {
		return ""
	}
	return e.cause.Error()
}

func newHealthError(code ErrorCode, message, remediation string, cause error) *HealthError {
	return &HealthError{Code: code, Message: message, Remediation: remediation, cause: cause}
}

// NewConfigError marks a config-validation problem (missing/invalid field)
// as a HealthError with ErrCodeInvalidConfig, so connectors that reject bad
// config up front (before ever dialing out) get the same structured
// treatment as a runtime failure. Connectors aren't required to use this -
// Classify already recognizes the plain fmt.Errorf strings they return
// today - but new connector code should prefer it.
func NewConfigError(message string) *HealthError {
	return newHealthError(ErrCodeInvalidConfig, message, "Fix the connection's configuration and save it again.", nil)
}

// Classify turns whatever error a connector, the SQL guard, or a guardrail
// returned into a HealthError with a stable code, a plain-language message,
// and a concrete next step. It is the one place this pattern-matching
// happens - applied centrally by Service.Test/Query/QueryAdhoc - rather
// than something every connector has to duplicate.
func Classify(err error) *HealthError {
	if err == nil {
		return nil
	}

	var he *HealthError
	if errors.As(err, &he) {
		return he
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return newHealthError(ErrCodeTimeout, "The request timed out.",
			"Check that the target is reachable and responds quickly enough - a slow query or an overloaded target can also cause this.", err)
	}
	if errors.Is(err, context.Canceled) {
		return newHealthError(ErrCodeTimeout, "The request was canceled before it finished.",
			"Try again; if this keeps happening, the target is likely too slow to respond within the configured timeout.", err)
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return classifyPostgres(pgErr, err)
	}
	var myErr *mysqldriver.MySQLError
	if errors.As(err, &myErr) {
		return classifyMySQL(myErr, err)
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return classifyAWS(apiErr, err)
	}
	var azErr *azcore.ResponseError
	if errors.As(err, &azErr) {
		return classifyHTTPStatus(azErr.StatusCode, err)
	}
	var gErr *googleapi.Error
	if errors.As(err, &gErr) {
		return classifyHTTPStatus(gErr.Code, err)
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return newHealthError(ErrCodeTimeout, "The connection attempt timed out.",
				"Check that the host and port are correct and reachable from this server (firewall, security group, VPC routing).", err)
		}
		return newHealthError(ErrCodeNetworkUnreachable, "Couldn't reach the target host.",
			"Check the host and port, and that this server can reach it over the network (firewall, security group, VPC routing, DNS).", err)
	}

	return classifyByMessage(err)
}

func classifyPostgres(pgErr *pgconn.PgError, cause error) *HealthError {
	switch pgErr.Code {
	case "28P01", "28000": // invalid_password / invalid_authorization_specification
		return newHealthError(ErrCodeAuthFailed, "The database rejected the username or password.",
			"Double-check the user and password configured on this connection.", cause)
	case "3D000": // invalid_catalog_name
		return newHealthError(ErrCodeNotFound, "That database name doesn't exist on this server.",
			"Check the database name for typos, or confirm it's been created.", cause)
	case "42501": // insufficient_privilege
		return newHealthError(ErrCodePermissionDenied, "The database user doesn't have permission for this.",
			"Grant the user SELECT on the tables it needs, or use a role with broader read access.", cause)
	}
	return newHealthError(ErrCodeUnknown, "The database returned an error.",
		"See the technical detail below, or check the database's own logs.", cause)
}

func classifyMySQL(myErr *mysqldriver.MySQLError, cause error) *HealthError {
	switch myErr.Number {
	case 1045: // ER_ACCESS_DENIED_ERROR
		return newHealthError(ErrCodeAuthFailed, "The database rejected the username or password.",
			"Double-check the user and password configured on this connection.", cause)
	case 1049: // ER_BAD_DB_ERROR
		return newHealthError(ErrCodeNotFound, "That database name doesn't exist on this server.",
			"Check the database name for typos, or confirm it's been created.", cause)
	case 1044, 1142: // ER_DBACCESS_DENIED_ERROR / ER_TABLEACCESS_DENIED_ERROR
		return newHealthError(ErrCodePermissionDenied, "The database user doesn't have permission for this.",
			"Grant the user SELECT on the tables it needs, or use a role with broader read access.", cause)
	}
	return newHealthError(ErrCodeUnknown, "The database returned an error.",
		"See the technical detail below, or check the database's own logs.", cause)
}

func classifyAWS(apiErr smithy.APIError, cause error) *HealthError {
	switch apiErr.ErrorCode() {
	case "AccessDenied", "AccessDeniedException", "UnauthorizedException":
		return newHealthError(ErrCodePermissionDenied, "AWS rejected this request due to insufficient IAM permissions.",
			"Add the specific action this service needs (e.g. athena:GetQueryResults, s3:GetObject) to the IAM role or user this connection uses.", cause)
	case "InvalidAccessKeyId", "SignatureDoesNotMatch", "UnrecognizedClientException", "ExpiredToken", "ExpiredTokenException":
		return newHealthError(ErrCodeAuthFailed, "AWS rejected the access key or session token.",
			"Check the access key ID/secret access key (or session token, if using temporary credentials) configured for this connection.", cause)
	case "ResourceNotFoundException", "NoSuchBucket", "NoSuchKey", "NotFoundException":
		return newHealthError(ErrCodeNotFound, "That AWS resource doesn't exist.",
			"Check the table/bucket/key/workgroup name for typos, and that it exists in the configured region.", cause)
	case "ThrottlingException", "TooManyRequestsException", "RequestLimitExceeded":
		return newHealthError(ErrCodeRateLimited, "AWS is rate-limiting this connection.",
			"Slow down how often this connection is queried, or request a service quota increase.", cause)
	}
	return newHealthError(ErrCodeUnknown, "AWS returned an error ("+apiErr.ErrorCode()+").",
		"See the technical detail below for what AWS reported.", cause)
}

func classifyHTTPStatus(status int, cause error) *HealthError {
	switch status {
	case 401:
		return newHealthError(ErrCodeAuthFailed, "The credentials on this connection were rejected.",
			"Double-check the credentials configured for this connection.", cause)
	case 403:
		return newHealthError(ErrCodePermissionDenied, "The credentials were accepted, but don't have permission for this.",
			"Grant the underlying identity the specific permission it needs.", cause)
	case 404:
		return newHealthError(ErrCodeNotFound, "The requested resource doesn't exist.",
			"Check the name/path/identifier for typos.", cause)
	case 429:
		return newHealthError(ErrCodeRateLimited, "The target is rate-limiting this connection.",
			"Slow down how often this connection is queried, or ask the provider to raise your quota.", cause)
	}
	return newHealthError(ErrCodeUnknown, "The request failed.",
		"See the technical detail below for what was returned.", cause)
}

// classifyByMessage is the last resort for drivers/libraries that don't
// expose a typed error Classify already knows how to unwrap - substring
// matching on the handful of phrasings that actually recur in practice.
func classifyByMessage(err error) *HealthError {
	msg := strings.ToLower(err.Error())
	switch {
	case containsAny(msg, "password authentication failed", "authentication failed", "invalid_grant", "invalid api key", "unauthorized", " 401"):
		return newHealthError(ErrCodeAuthFailed, "The credentials on this connection were rejected.",
			"Double-check the username/password, API key, or token configured for this connection.", err)
	case containsAny(msg, "access denied", "forbidden", "permission denied", " 403"):
		return newHealthError(ErrCodePermissionDenied, "The credentials were accepted, but don't have permission for this.",
			"Grant the underlying user/role/service account the specific permission it needs.", err)
	case containsAny(msg, "no such host", "connection refused", "network is unreachable", "no route to host"):
		return newHealthError(ErrCodeNetworkUnreachable, "Couldn't reach the target host.",
			"Check the host and port, and that this server can reach it over the network (firewall, security group, VPC routing, DNS).", err)
	case containsAny(msg, "context deadline exceeded", "i/o timeout", "timed out"):
		return newHealthError(ErrCodeTimeout, "The request timed out.",
			"Check that the target is reachable and responds quickly enough.", err)
	case containsAny(msg, "does not exist", "not found", " 404"):
		return newHealthError(ErrCodeNotFound, "The requested resource doesn't exist.",
			"Check the name/path/identifier for typos.", err)
	case containsAny(msg, "rate limit", "too many requests", "throttl", " 429"):
		return newHealthError(ErrCodeRateLimited, "The target is rate-limiting this connection.",
			"Slow down how often this connection is queried, or ask the provider to raise your quota.", err)
	case containsAny(msg, "is required", "must be a valid", "invalid config", "invalid configuration", "configure authentication"):
		return newHealthError(ErrCodeInvalidConfig, "The connection configuration is invalid.",
			"Review the connection form for missing or invalid fields, then save it again.", err)
	}
	return newHealthError(ErrCodeUnknown, "Something went wrong reaching this connection.",
		"See the technical detail below for what the underlying system reported.", err)
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
