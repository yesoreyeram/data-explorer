package connections

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/aws/smithy-go"
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/api/googleapi"
)

type fakeTimeoutErr struct{ msg string }

func (e *fakeTimeoutErr) Error() string   { return e.msg }
func (e *fakeTimeoutErr) Timeout() bool   { return true }
func (e *fakeTimeoutErr) Temporary() bool { return true }

type fakeNetErr struct{ msg string }

func (e *fakeNetErr) Error() string   { return e.msg }
func (e *fakeNetErr) Timeout() bool   { return false }
func (e *fakeNetErr) Temporary() bool { return false }

func TestClassifyNil(t *testing.T) {
	if Classify(nil) != nil {
		t.Fatal("expected Classify(nil) to return nil")
	}
}

func TestClassifyPassesThroughExistingHealthError(t *testing.T) {
	original := NewConfigError("bad config")
	got := Classify(original)
	if got != original {
		t.Fatalf("expected Classify to return the same *HealthError unchanged, got %+v", got)
	}
}

func TestClassifyContextErrors(t *testing.T) {
	if got := Classify(context.DeadlineExceeded); got.Code != ErrCodeTimeout {
		t.Fatalf("expected ErrCodeTimeout for DeadlineExceeded, got %q", got.Code)
	}
	if got := Classify(context.Canceled); got.Code != ErrCodeTimeout {
		t.Fatalf("expected ErrCodeTimeout for Canceled, got %q", got.Code)
	}
	if got := Classify(fmt.Errorf("wrapped: %w", context.DeadlineExceeded)); !errors.Is(got, context.DeadlineExceeded) {
		t.Fatalf("expected classified error to still unwrap to context.DeadlineExceeded, got %v", got)
	}
}

func TestClassifyPostgres(t *testing.T) {
	cases := []struct {
		code string
		want ErrorCode
	}{
		{"28P01", ErrCodeAuthFailed},
		{"28000", ErrCodeAuthFailed},
		{"3D000", ErrCodeNotFound},
		{"42501", ErrCodePermissionDenied},
		{"XXUNK", ErrCodeUnknown},
	}
	for _, c := range cases {
		err := &pgconn.PgError{Code: c.code, Message: "boom"}
		got := Classify(err)
		if got.Code != c.want {
			t.Errorf("pg code %s: expected %q, got %q", c.code, c.want, got.Code)
		}
		if !errors.Is(got, err) {
			t.Errorf("pg code %s: expected classified error to unwrap to the original pgconn.PgError", c.code)
		}
	}
}

func TestClassifyMySQL(t *testing.T) {
	cases := []struct {
		number uint16
		want   ErrorCode
	}{
		{1045, ErrCodeAuthFailed},
		{1049, ErrCodeNotFound},
		{1044, ErrCodePermissionDenied},
		{1142, ErrCodePermissionDenied},
		{9999, ErrCodeUnknown},
	}
	for _, c := range cases {
		err := &mysqldriver.MySQLError{Number: c.number, Message: "boom"}
		got := Classify(err)
		if got.Code != c.want {
			t.Errorf("mysql number %d: expected %q, got %q", c.number, c.want, got.Code)
		}
	}
}

func TestClassifyAWS(t *testing.T) {
	cases := []struct {
		code string
		want ErrorCode
	}{
		{"AccessDenied", ErrCodePermissionDenied},
		{"UnauthorizedException", ErrCodePermissionDenied},
		{"InvalidAccessKeyId", ErrCodeAuthFailed},
		{"ExpiredToken", ErrCodeAuthFailed},
		{"NoSuchBucket", ErrCodeNotFound},
		{"ThrottlingException", ErrCodeRateLimited},
		{"SomethingElse", ErrCodeUnknown},
	}
	for _, c := range cases {
		err := &smithy.GenericAPIError{Code: c.code, Message: "boom"}
		got := Classify(err)
		if got.Code != c.want {
			t.Errorf("aws code %s: expected %q, got %q", c.code, c.want, got.Code)
		}
	}
}

func TestClassifyAzureAndGCPHTTPStatus(t *testing.T) {
	statusCases := []struct {
		status int
		want   ErrorCode
	}{
		{401, ErrCodeAuthFailed},
		{403, ErrCodePermissionDenied},
		{404, ErrCodeNotFound},
		{429, ErrCodeRateLimited},
		{500, ErrCodeUnknown},
	}
	for _, c := range statusCases {
		azErr := &azcore.ResponseError{StatusCode: c.status, ErrorCode: "boom"}
		if got := Classify(azErr); got.Code != c.want {
			t.Errorf("azure status %d: expected %q, got %q", c.status, c.want, got.Code)
		}

		gErr := &googleapi.Error{Code: c.status, Message: "boom"}
		if got := Classify(gErr); got.Code != c.want {
			t.Errorf("gcp status %d: expected %q, got %q", c.status, c.want, got.Code)
		}
	}
}

func TestClassifyNetError(t *testing.T) {
	var timeoutErr net.Error = &fakeTimeoutErr{msg: "dial timeout"}
	if got := Classify(timeoutErr); got.Code != ErrCodeTimeout {
		t.Fatalf("expected ErrCodeTimeout for a timing-out net.Error, got %q", got.Code)
	}

	var unreachableErr net.Error = &fakeNetErr{msg: "connection refused"}
	if got := Classify(unreachableErr); got.Code != ErrCodeNetworkUnreachable {
		t.Fatalf("expected ErrCodeNetworkUnreachable for a non-timing-out net.Error, got %q", got.Code)
	}
}

func TestClassifyByMessageFallback(t *testing.T) {
	cases := []struct {
		msg  string
		want ErrorCode
	}{
		{"password authentication failed for user", ErrCodeAuthFailed},
		{"403 Forbidden", ErrCodePermissionDenied},
		{"dial tcp: no such host", ErrCodeNetworkUnreachable},
		{"context deadline exceeded while waiting", ErrCodeTimeout},
		{"relation \"foo\" does not exist", ErrCodeNotFound},
		{"429 Too Many Requests", ErrCodeRateLimited},
		{"something completely unrecognized happened", ErrCodeUnknown},
	}
	for _, c := range cases {
		got := Classify(errors.New(c.msg))
		if got.Code != c.want {
			t.Errorf("message %q: expected %q, got %q", c.msg, c.want, got.Code)
		}
	}
}

func TestHealthErrorDetailAndError(t *testing.T) {
	cause := errors.New("underlying driver error")
	he := newHealthError(ErrCodeUnknown, "Something went wrong.", "Try again.", cause)
	if he.Detail() != cause.Error() {
		t.Fatalf("expected Detail() to return the cause's message, got %q", he.Detail())
	}
	if he.Error() != "Something went wrong.: underlying driver error" {
		t.Fatalf("unexpected Error() string: %q", he.Error())
	}

	noCause := newHealthError(ErrCodeInvalidConfig, "Bad config.", "Fix it.", nil)
	if noCause.Detail() != "" {
		t.Fatalf("expected empty Detail() with no cause, got %q", noCause.Detail())
	}
	if noCause.Error() != "Bad config." {
		t.Fatalf("unexpected Error() string with no cause: %q", noCause.Error())
	}
}
