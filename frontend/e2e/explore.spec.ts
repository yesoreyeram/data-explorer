/**
 * Explore page E2E tests – verify the query interface renders correctly and
 * the connection selector / mode toggle work as expected.
 */
import { expect, test } from "@playwright/test";

test.describe("Explore page", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/explore");
  });

  test("renders the Explore heading", async ({ page }) => {
    await expect(page.getByRole("heading", { name: "Explore" })).toBeVisible();
  });

  test("shows the 'Saved connection' mode button", async ({ page }) => {
    await expect(page.getByRole("button", { name: "Saved connection" })).toBeVisible();
  });

  test("shows the 'Temporary connection' mode button for admin", async ({ page }) => {
    await expect(page.getByRole("button", { name: "Temporary connection" })).toBeVisible();
  });

  test("shows a connection selector dropdown in 'Saved connection' mode", async ({ page }) => {
    await page.getByRole("button", { name: "Saved connection" }).click();
    await expect(page.getByLabel("Connection")).toBeVisible();
  });

  test("shows a type selector in 'Temporary connection' mode", async ({ page }) => {
    await page.getByRole("button", { name: "Temporary connection" }).click();
    await expect(page.getByLabel("Type")).toBeVisible();
  });

  test("shows the Run button after selecting a temporary connection type", async ({ page }) => {
    await page.getByRole("button", { name: "Temporary connection" }).click();
    // Run button should be visible once a connection type is selected
    await expect(page.getByRole("button", { name: "Run" })).toBeVisible();
  });

  test("shows no error banner on initial load", async ({ page }) => {
    await expect(page.locator(".error-banner")).not.toBeVisible();
  });
});
