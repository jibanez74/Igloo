import { expect, test } from "@playwright/test";
import { isExpectedUnauthorizedResourceMessage } from "./e2e-browser-issues";

test.describe("production startup", () => {
  test.skip(process.env.E2E_PRODUCTION !== "1", "Requires built production assets");
  test.setTimeout(60_000);

  for (const pathname of ["/", "/login"]) {
    test(`boots from ${pathname} and survives a reload`, async ({ page }) => {
      const issues: string[] = [];

      // Register before navigation so failures before React mounts are captured.
      page.on("pageerror", error => issues.push(error.stack ?? error.message));
      page.on("requestfailed", request => {
        issues.push(`${request.method()} ${request.url()}: ${request.failure()?.errorText}`);
      });
      page.on("response", response => {
        const expectedUnauthorized = response.status() === 401 &&
          response.request().method() === "GET" &&
          new URL(response.url()).pathname === "/api/auth/user";
        if (response.status() >= 400 && !expectedUnauthorized) {
          issues.push(`${response.status()} ${response.url()}`);
        }
      });
      page.on("console", message => {
        if (message.type() !== "error") return;
        if (
          isExpectedUnauthorizedResourceMessage(message.text()) &&
          new URL(message.location().url, page.url()).pathname === "/api/auth/user"
        ) return;
        issues.push(message.text());
      });

      try {
        await page.goto(pathname, { waitUntil: "networkidle" });
        for (const reload of [false, true]) {
          if (reload) await page.reload({ waitUntil: "networkidle" });

          expect(issues).toEqual([]);
          await expect(page).toHaveURL(/\/login(?:\?|$)/);
          await expect(page.getByLabel("Email")).toBeVisible();
          await expect(page.getByLabel("Password", { exact: true })).toBeVisible();
          await expect(page.getByRole("button", { name: "Sign in", exact: true })).toBeVisible();
          await expect(page.locator("html")).toHaveAttribute("data-app-ready", "true");
          await expect(page.locator("#initial-splash")).toHaveCount(0);
        }
      } finally {
        expect(issues, "Browser errors during production startup").toEqual([]);
      }
    });
  }
});
