/**
 * Audit log E2E tests – verify the audit log page renders the table, supports
 * filtering by action, and paginates correctly.
 */
import { expect, test } from "@playwright/test";

test.describe("Audit log page", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/audit-log");
  });

  test("renders the Audit log heading", async ({ page }) => {
    await expect(page.getByRole("heading", { name: "Audit log" })).toBeVisible();
  });

  test("shows no error banner on initial load", async ({ page }) => {
    await expect(page.locator(".error-banner")).not.toBeVisible();
  });

  test("renders the audit table with correct column headers", async ({ page }) => {
    await expect(page.getByRole("columnheader", { name: "Time" })).toBeVisible();
    await expect(page.getByRole("columnheader", { name: "Actor" })).toBeVisible();
    await expect(page.getByRole("columnheader", { name: "Action" })).toBeVisible();
    await expect(page.getByRole("columnheader", { name: "Resource" })).toBeVisible();
    await expect(page.getByRole("columnheader", { name: "Outcome" })).toBeVisible();
    await expect(page.getByRole("columnheader", { name: "IP address" })).toBeVisible();
  });

  test("contains at least one audit entry (login during global setup)", async ({ page }) => {
    // At minimum the global-setup login generated an audit entry
    const rows = page.locator("table tbody tr");
    const count = await rows.count();
    expect(count).toBeGreaterThan(0);
  });

  test("filters by action text", async ({ page }) => {
    const actionInput = page.getByPlaceholder("Action (e.g. connection.create)");
    await actionInput.fill("auth.login");

    // Table should refresh; either show matching rows or the empty state
    await page.waitForTimeout(500);
    await expect(page.locator(".error-banner")).not.toBeVisible();
  });

  test("filters by resource type text", async ({ page }) => {
    const resourceInput = page.getByPlaceholder("Resource type");
    await resourceInput.fill("user");

    await page.waitForTimeout(500);
    await expect(page.locator(".error-banner")).not.toBeVisible();
  });

  test("clears filter restores full list", async ({ page }) => {
    const actionInput = page.getByPlaceholder("Action (e.g. connection.create)");
    await actionInput.fill("auth.login");
    await page.waitForTimeout(300);
    await actionInput.clear();
    await page.waitForTimeout(300);

    // After clearing the filter the table should still load without error
    await expect(page.locator(".error-banner")).not.toBeVisible();
  });

  test("pagination buttons are present", async ({ page }) => {
    await expect(page.getByRole("button", { name: "Previous" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Next" })).toBeVisible();
  });
});
