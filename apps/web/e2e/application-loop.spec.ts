import { expect, test } from "@playwright/test";

const email = process.env.E2E_USER_EMAIL;
const password = process.env.E2E_USER_PASSWORD;

test("staging application loop: browse, save, and advance an application", async ({ page }) => {
  test.skip(!email || !password, "Set E2E_USER_EMAIL and E2E_USER_PASSWORD for seeded staging flow.");

  await page.goto("/login");
  await page.getByLabel("Email").fill(email!);
  await page.getByLabel("Password").fill(password!);
  await page.getByRole("button", { name: "Log in" }).click();
  await expect(page).toHaveURL(/\/(dashboard|onboarding)$/);

  await page.goto("/jobs");
  await page.getByRole("button", { name: "Latest 24h" }).click();
  const firstJob = page.locator('a[href^="/jobs/"]').first();
  test.skip((await firstJob.count()) === 0, "Staging environment has no fresh jobs.");
  await firstJob.click();

  await expect(page.getByRole("button", { name: /Save Job|Saved/ })).toBeVisible();
  const saveButton = page.getByRole("button", { name: "Save Job" });
  if (await saveButton.isVisible()) {
    await saveButton.click();
    await expect(page.getByRole("button", { name: "Saved ✓" })).toBeVisible();
  }

  await page.goto("/applications");
  const card = page.locator("[data-application-id]").first();
  await expect(card).toBeVisible();

  const move = card.getByRole("button", { name: /^Move to / });
  if (await move.isVisible()) {
    const destination = (await move.textContent())?.replace("Move to ", "").replace(" →", "").trim();
    await move.click();
    if (destination) {
      await expect(page.getByText(new RegExp(`^${destination} \\(\\d+\\)$`, "i"))).toBeVisible();
    }
  }
});
