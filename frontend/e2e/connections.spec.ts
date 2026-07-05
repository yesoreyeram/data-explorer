/**
 * Connections E2E tests – cover listing, creating, testing, and deleting a
 * REST connection as an admin user.
 */
import { expect, test } from "@playwright/test";

const TEST_CONN_NAME = `e2e-rest-${Date.now()}`;

test.describe("Connections page", () => {
  test("loads the connections list without error", async ({ page }) => {
    await page.goto("/connections");
    await expect(page.getByRole("heading", { name: "Connections" })).toBeVisible();
    await expect(page.locator(".error-banner")).not.toBeVisible();
  });

  test("shows 'New connection' and 'Browse catalog' buttons for admin", async ({ page }) => {
    await page.goto("/connections");
    await expect(page.getByRole("button", { name: "New connection" })).toBeVisible();
    await expect(page.getByRole("button", { name: "Browse catalog" })).toBeVisible();
  });

  test("opens the connection form modal on 'New connection'", async ({ page }) => {
    await page.goto("/connections");
    await page.getByRole("button", { name: "New connection" }).click();
    // Modal should appear with a Name field
    await expect(page.getByLabel("Name")).toBeVisible();
  });

  test("creates a REST connection, verifies it appears in the list, then deletes it", async ({ page }) => {
    await page.goto("/connections");
    await page.getByRole("button", { name: "New connection" }).click();

    // Fill in the form – select "REST API" type
    await page.getByLabel("Name").fill(TEST_CONN_NAME);
    const typeSelect = page.getByLabel("Type");
    await typeSelect.selectOption("rest");

    // Set a base URL (required for REST connector)
    const baseUrlInput = page.getByLabel("Base URL");
    await baseUrlInput.fill("https://jsonplaceholder.typicode.com");

    // Submit
    await page.getByRole("button", { name: "Save" }).click();

    // Connection should now appear in the table
    await expect(page.getByRole("cell", { name: TEST_CONN_NAME })).toBeVisible();

    // Delete the connection
    const row = page.getByRole("row", { name: new RegExp(TEST_CONN_NAME) });
    await row.getByLabel("Delete").click();

    // Confirm the browser dialog
    page.once("dialog", (dialog) => dialog.accept());

    // Row should disappear
    await expect(page.getByRole("cell", { name: TEST_CONN_NAME })).not.toBeVisible();
  });

  test("opens the catalog browser modal", async ({ page }) => {
    await page.goto("/connections");
    await page.getByRole("button", { name: "Browse catalog" }).click();
    // Catalog modal has a search or list of integrations
    await expect(page.getByRole("dialog")).toBeVisible();
  });
});
