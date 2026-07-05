/**
 * Global E2E setup: register a test admin user (if not already present),
 * elevate them to the "admin" role via the database, then log in and save the
 * browser storage state so every authenticated test can reuse the session.
 *
 * Environment variables consumed:
 *   E2E_ADMIN_EMAIL    (default: e2e-admin@test.local)
 *   E2E_ADMIN_PASSWORD (default: e2e-test-password-secure123)
 *   DATABASE_URL       (optional – if set, psql is used to elevate the role)
 *   PLAYWRIGHT_BASE_URL (default: http://localhost:5173)
 */
import { execSync } from "child_process";
import * as fs from "fs";
import * as path from "path";

import { expect, test as setup } from "@playwright/test";

import { ADMIN_EMAIL, ADMIN_PASSWORD } from "./constants.ts";

export { ADMIN_EMAIL, ADMIN_PASSWORD };

const ADMIN_NAME = "E2E Admin";
const BASE_URL = process.env.PLAYWRIGHT_BASE_URL ?? "http://localhost:5173";

setup("create and authenticate admin test user", async ({ page, request }) => {
  // 1. Register (idempotent – 409 Conflict is fine if already exists).
  const registerRes = await request.post(`${BASE_URL}/api/v1/auth/register`, {
    data: { email: ADMIN_EMAIL, displayName: ADMIN_NAME, password: ADMIN_PASSWORD },
    failOnStatusCode: false,
  });
  expect([200, 201, 409]).toContain(registerRes.status());

  // 2. Elevate to admin via psql when DATABASE_URL is present (CI).
  const databaseUrl = process.env.DATABASE_URL;
  if (databaseUrl) {
    const sql = `
      INSERT INTO user_roles (user_id, role_id)
      SELECT u.id, r.id
      FROM users u
      CROSS JOIN roles r
      WHERE u.email = '${ADMIN_EMAIL}'
        AND r.name = 'admin'
      ON CONFLICT DO NOTHING;
    `;
    try {
      execSync(`psql "${databaseUrl}" -c "${sql.replace(/\n/g, " ").trim()}"`, { stdio: "pipe" });
    } catch {
      // psql may not be in PATH in some environments; tests degrade gracefully.
    }
  }

  // 3. Log in and capture session cookies.
  await page.goto(`${BASE_URL}/login`);
  await page.locator("#email").fill(ADMIN_EMAIL);
  await page.locator("#password").fill(ADMIN_PASSWORD);
  await page.locator('button[type="submit"]').click();
  await page.waitForURL(`${BASE_URL}/`);
  await expect(page).toHaveURL(`${BASE_URL}/`);

  const authDir = path.join(__dirname, ".auth");
  fs.mkdirSync(authDir, { recursive: true });
  await page.context().storageState({ path: path.join(authDir, "admin.json") });
});
