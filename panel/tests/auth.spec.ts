import { expect, test } from "@playwright/test";

test("anonymous visit is redirected to login", async ({ page }) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/login/);
  await expect(page.getByRole("heading", { name: "Entrar", exact: true })).toBeVisible();
  await expect(page.getByRole("button", { name: "Continuar com Google" })).toBeVisible();
});

test("onboarding requires a session", async ({ page }) => {
  await page.goto("/onboarding");
  await expect(page).toHaveURL(/\/login/);
});

test("bridge proxy rejects anonymous calls", async ({ request }) => {
  const response = await request.get("/api/bridge/tenants/default/stats");
  expect(response.status()).toBe(401);
  const payload = (await response.json()) as { success?: boolean; error?: { code?: string } };
  expect(payload.success).toBe(false);
  expect(payload.error?.code).toBe("UNAUTHORIZED");
});

test("login page exposes the Google provider", async ({ page }) => {
  await page.goto("/login");
  await expect(page.getByText("Use sua conta Google")).toBeVisible();
});
