import { expect, test } from "@playwright/test";
import { isExpectedUnauthorizedResourceMessage } from "./e2e-browser-issues";

test.describe("production startup", () => {
  test.skip(process.env.E2E_PRODUCTION !== "1", "Requires built production assets");

  for (const pathname of ["/", "/login"]) {
    test(`boots from ${pathname} and survives a reload`, async ({ page }) => {
      const issues: string[] = [];

      // Stricter than trackBrowserIssues: any failed request, any non-/api
      // error status and any console error outside the one expected 401 fails
      // the production build check.
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

  // The signed-out page loads only what it shows (docs/design-system.md
  // §1.3): the Latin face of Inter, not the subsets for other scripts, and
  // none of the music player, which loads with the signed-in shell. The
  // player's now-playing announcement marks its code.
  test("loads the Latin font face and no player on /login", async ({ page }) => {
    const fonts: string[] = [];
    const scripts: Promise<string>[] = [];
    page.on("response", response => {
      const path = new URL(response.url()).pathname;
      if (path.endsWith(".woff2")) fonts.push(path);
      if (path.endsWith(".js")) scripts.push(response.text());
    });

    await page.goto("/login", { waitUntil: "networkidle" });
    await expect(page.getByRole("button", { name: "Sign in", exact: true })).toBeVisible();
    expect(await page.evaluate(async () => {
      await document.fonts.ready;
      return document.fonts.check("16px Inter");
    })).toBe(true);

    expect(fonts).toEqual([expect.stringMatching(/^\/assets\/inter-latin-[\w-]+\.woff2$/)]);
    const playerScripts = (await Promise.all(scripts)).filter(body =>
      body.includes("Now playing: "),
    );
    expect(playerScripts).toEqual([]);
  });

  // The component gallery is a development page (src/routes/gallery.tsx). Its
  // route stays in the build so the route tree is the same in every mode, but
  // it must send a visitor home, here on to the sign-in page, and never render.
  test("does not serve the component gallery", async ({ page }) => {
    await page.goto("/gallery", { waitUntil: "networkidle" });

    await expect(page).toHaveURL(/\/login(?:\?|$)/);
    await expect(page.getByLabel("Email")).toBeVisible();
    await expect(page.locator("[data-gallery-root]")).toHaveCount(0);
  });
});
