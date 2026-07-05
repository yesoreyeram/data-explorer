/**
 * Shared constants for E2E tests. Centralising them here avoids circular
 * imports between the global setup and spec files.
 */
export const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL ?? "e2e-admin@test.local";
export const ADMIN_PASSWORD = process.env.E2E_ADMIN_PASSWORD ?? "e2e-test-password-secure123";
