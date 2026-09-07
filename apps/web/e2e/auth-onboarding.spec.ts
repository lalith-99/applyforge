import { expect, test } from "@playwright/test";

test("email signup completes onboarding and returns as an onboarded user", async ({ page }) => {
  const email = `applyforge-e2e+${Date.now()}@example.com`;
  const password = "ApplyForge-E2E-123!";

  await page.goto("/signup");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Create account" }).click();

  await expect(page).toHaveURL(/\/onboarding$/);
  await page.getByLabel("First name").fill("E2E");
  await page.getByLabel("Last name").fill("Candidate");
  await page.getByLabel("City").fill("Austin");
  await page.getByLabel("State").fill("TX");
  await page.getByLabel("Country").fill("United States");
  await page.getByLabel("Primary target titles (comma separated)").fill(
    "Backend Engineer, Software Engineer",
  );
  await page.getByLabel("Seniority").fill("Senior");
  await page.getByLabel("Years of experience").fill("6");
  await page.getByLabel("Preferred technologies (comma separated)").fill(
    "Go, Java, Kafka, AWS, Kubernetes",
  );
  await page.getByRole("button", { name: "Continue" }).click();

  await expect(page.getByText("Step 2 of 2")).toBeVisible();
  await page.getByLabel("Remote").check();
  await page.getByLabel("Full-time").check();
  await page.getByLabel("Current immigration status").fill("H1B");
  await page.getByLabel("Requires H-1B transfer support").check();
  await page.getByLabel("Prefer green-card sponsorship support").check();
  await page.getByLabel("Prefer PERM sponsorship pathway").check();
  await page.getByRole("button", { name: "Finish onboarding" }).click();

  await expect(page).toHaveURL(/\/dashboard$/);

  // A returning login must not be routed back through onboarding.
  const apiBase = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080/api/v1";
  await page.request.post(`${apiBase}/auth/logout`);
  await page.goto("/login");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill(password);
  await page.getByRole("button", { name: "Log in" }).click();
  await expect(page).toHaveURL(/\/dashboard$/);
});
