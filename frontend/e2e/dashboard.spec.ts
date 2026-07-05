/**
 * Dashboard E2E tests – verify the main overview page loads correctly for an
 * authenticated user, shows the stat grid, and surfaces the connections and
 * workflow summary cards.
 */
import { expect, test } from "@playwright/test";

test.describe("Dashboard", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
  });

  test("renders the dashboard without error", async ({ page }) => {
    await expect(page).toHaveURL("/");
    // No full-page error banner
    await expect(page.locator(".error-banner")).not.toBeVisible();
  });

  test("shows the stat grid with connection and workflow tiles", async ({ page }) => {
    // StatTile labels are always present regardless of data
    await expect(page.getByText("Connections")).toBeVisible();
    await expect(page.getByText("Healthy connections")).toBeVisible();
    await expect(page.getByText("Workflows")).toBeVisible();
  });

  test("shows the recent connections card", async ({ page }) => {
    await expect(page.getByText("Recent connections")).toBeVisible();
  });

  test("shows the recent workflows card", async ({ page }) => {
    await expect(page.getByText("Recent workflows")).toBeVisible();
  });

  test("shows the saved charts card", async ({ page }) => {
    await expect(page.getByText("Saved charts")).toBeVisible();
  });

  test("navigates to /connections via 'View all' link", async ({ page }) => {
    await page.getByRole("link", { name: "View all" }).first().click();
    await expect(page).toHaveURL("/connections");
  });
});
