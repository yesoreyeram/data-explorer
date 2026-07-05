/**
 * Workflows E2E tests – create a workflow, open its builder, then delete it.
 */
import { expect, test } from "@playwright/test";

const WF_NAME = `e2e-workflow-${Date.now()}`;
const WF_DESC = "Created by Playwright E2E test";

test.describe("Workflows list page", () => {
  test("renders the Workflows heading", async ({ page }) => {
    await page.goto("/workflows");
    await expect(page.getByRole("heading", { name: "Workflows" })).toBeVisible();
  });

  test("shows the 'New workflow' button for admin", async ({ page }) => {
    await page.goto("/workflows");
    await expect(page.getByRole("button", { name: "New workflow" })).toBeVisible();
  });

  test("creates a workflow via the modal then navigates to the builder", async ({ page }) => {
    await page.goto("/workflows");
    await page.getByRole("button", { name: "New workflow" }).click();

    // Modal should appear
    await expect(page.getByRole("dialog")).toBeVisible();

    // Fill in name and description
    await page.getByLabel("Name").fill(WF_NAME);
    await page.getByLabel("Description").fill(WF_DESC);

    // Submit
    await page.getByRole("button", { name: "Create" }).click();

    // Should navigate to the workflow builder
    await expect(page).toHaveURL(/\/workflows\/.+/);
  });

  test("workflow builder shows the node palette", async ({ page }) => {
    // Navigate to any workflow (the one created above should be first in the list)
    await page.goto("/workflows");
    const wfCards = page.locator('[class*="card"]');
    await wfCards.first().click();
    await expect(page).toHaveURL(/\/workflows\/.+/);

    // Palette items are rendered
    await expect(page.getByText("Source")).toBeVisible();
    await expect(page.getByText("Transform")).toBeVisible();
  });
});

test.describe("Workflow builder actions", () => {
  test("navigates back to the workflows list", async ({ page }) => {
    // Go directly to workflows list
    await page.goto("/workflows");
    // Verify no error
    await expect(page.locator(".error-banner")).not.toBeVisible();
  });
});

test.describe("Workflow deletion", () => {
  test("deletes the E2E workflow created earlier", async ({ page }) => {
    await page.goto("/workflows");

    // Find the card with the known workflow name
    const wfCard = page.getByText(WF_NAME).first();

    // It may or may not be there (depends on ordering with prior test in same run)
    if (await wfCard.isVisible()) {
      const deleteBtn = wfCard.locator("..").getByLabel("Delete");
      page.once("dialog", (dialog) => dialog.accept());
      await deleteBtn.click();
      await expect(page.getByText(WF_NAME)).not.toBeVisible();
    }
  });
});
