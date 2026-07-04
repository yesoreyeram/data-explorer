-- Structured health-check results, alongside the existing plain-text
-- last_error, so the API/UI can render a stable error code + an actionable
-- remediation instead of just a driver's raw message.

ALTER TABLE connections ADD COLUMN last_error_code        TEXT NOT NULL DEFAULT '';
ALTER TABLE connections ADD COLUMN last_error_remediation  TEXT NOT NULL DEFAULT '';
ALTER TABLE connections ADD COLUMN last_check_duration_ms  BIGINT NOT NULL DEFAULT 0;
