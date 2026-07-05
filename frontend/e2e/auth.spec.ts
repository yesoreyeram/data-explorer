/**
 * Auth E2E tests – run without saved auth state so we can test the
 * unauthenticated flows (redirect, error handling, register link).
 */
import { expect, test } from "@playwright/test";

// These tests deliberately start with no session.
test.use({ storageState: { cookies: [], origins: [] } });

test.describe("Login page", () => {
  test("renders sign-in form", async ({ page }) => {
    await page.goto("/login");
    await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible();
    await expect(page.getByLabel("Email")).toBeVisible();
    await expect(page.getByLabel("Password")).toBeVisible();
    await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
  });

  test("has a link to the register page", async ({ page }) => {
    await page.goto("/login");
    await page.getByRole("link", { name: "Create one" }).click();
    await expect(page).toHaveURL(/\/register/);
  });

  test("shows an error banner for invalid credentials", async ({ page }) => {
    await page.goto("/login");
    await page.getByLabel("Email").fill("nobody@example.invalid");
    await page.getByLabel("Password").fill("wrongpassword9999");
    await page.getByRole("button", { name: "Sign in" }).click();
    await expect(page.locator(".error-banner")).toBeVisible();
  });

  test("redirects unauthenticated users from / to /login", async ({ page }) => {
    await page.goto("/");
    await expect(page).toHaveURL(/\/login/);
  });

  test("redirects unauthenticated users from /connections to /login", async ({ page }) => {
    await page.goto("/connections");
    await expect(page).toHaveURL(/\/login/);
  });
});

test.describe("Register page", () => {
  test("renders registration form", async ({ page }) => {
    await page.goto("/register");
    await expect(page.getByRole("heading", { name: "Create account" })).toBeVisible();
    await expect(page.getByLabel("Full name")).toBeVisible();
    await expect(page.getByLabel("Email")).toBeVisible();
    await expect(page.getByLabel("Password")).toBeVisible();
  });

  test("has a link back to the login page", async ({ page }) => {
    await page.goto("/register");
    await page.getByRole("link", { name: "Sign in" }).click();
    await expect(page).toHaveURL(/\/login/);
  });

  test("shows an error for a duplicate email", async ({ page }) => {
    // Use the known admin test account created in global.setup.ts.
    const { ADMIN_EMAIL, ADMIN_PASSWORD } = await import("./constants.ts");
    await page.goto("/register");
    await page.getByLabel("Full name").fill("Duplicate User");
    await page.getByLabel("Email").fill(ADMIN_EMAIL);
    await page.getByLabel("Password").fill(ADMIN_PASSWORD);
    await page.getByRole("button", { name: "Create account" }).click();
    await expect(page.locator(".error-banner")).toBeVisible();
  });
});
