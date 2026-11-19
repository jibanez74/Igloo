import { expect, test } from "./e2e-test";
import type { Page } from "@playwright/test";
import {
  isExpectedUnauthorizedResourceMessage,
  signedOutIssueRules,
} from "./e2e-browser-issues";
import { loginPageViaApi } from "./e2e-auth";

/**
 * Stricter than the shared browser-issue net: any failed request, any error
 * status outside /api/ and any console error fails the production build
 * check. Signed out, the shell's one auth check may answer 401. Register it
 * before navigating, so failures before React mounts are caught.
 */
function trackStartupIssues(page: Page, { signedOut }: { signedOut: boolean }) {
  const issues: string[] = [];
  const isAuthCheck = (url: string) => new URL(url, page.url()).pathname === "/api/auth/user";

  page.on("pageerror", error => issues.push(error.stack ?? error.message));
  page.on("requestfailed", request => {
    issues.push(`${request.method()} ${request.url()}: ${request.failure()?.errorText}`);
  });
  page.on("response", response => {
    const expectedUnauthorized = signedOut &&
      response.status() === 401 &&
      response.request().method() === "GET" &&
      isAuthCheck(response.url());
    if (response.status() >= 400 && !expectedUnauthorized) {
      issues.push(`${response.status()} ${response.url()}`);
    }
  });
  page.on("console", message => {
    if (message.type() !== "error") return;
    if (
      signedOut &&
      isExpectedUnauthorizedResourceMessage(message.text()) &&
      isAuthCheck(message.location().url)
    ) return;
    issues.push(message.text());
  });
  return issues;
}

test.describe("production startup", () => {
  test.skip(process.env.E2E_PRODUCTION !== "1", "Requires built production assets");
  // Every test here but one starts signed out.
  test.use({ browserIssueOptions: signedOutIssueRules });

  for (const pathname of ["/", "/login"]) {
    test(`boots from ${pathname} and survives a reload`, async ({ page }) => {
      const issues = trackStartupIssues(page, { signedOut: true });

      // networkidle on purpose: the issue list is read once every startup
      // request has finished, so a late chunk or asset failure is in it.
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

  // A signed-in reader's link or bookmark loads the app straight from a
  // nested address, and the server must answer it with the app whose assets
  // still load from the root. An address no page claims boots into the
  // shell's not-found page rather than the server's or the router's own.
  test("boots signed in from a nested address and from an unknown one", async ({ page }) => {
    await loginPageViaApi(page);
    const issues = trackStartupIssues(page, { signedOut: false });
    const pages = [
      { pathname: "/settings/account", heading: "Settings", title: "Account Settings - Igloo" },
      { pathname: "/movies/7/nope", heading: "Page not found", title: "Page not found - Igloo" },
    ];

    try {
      for (const { pathname, heading, title } of pages) {
        // networkidle on purpose, as above.
        await page.goto(pathname, { waitUntil: "networkidle" });
        for (const reload of [false, true]) {
          if (reload) await page.reload({ waitUntil: "networkidle" });

          expect(issues).toEqual([]);
          await expect(page).toHaveURL(new RegExp(`${pathname}$`));
          await expect(page.getByRole("heading", { level: 1, name: heading })).toBeVisible();
          await expect(page).toHaveTitle(title);
          await expect(page.getByRole("navigation", { name: "Main navigation" })).toBeVisible();
          await expect(page.locator("#initial-splash")).toHaveCount(0);
        }
      }
    } finally {
      expect(issues, "Browser errors during production startup").toEqual([]);
    }
  });

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

    // networkidle on purpose: the font and script lists are complete only
    // once the page has stopped loading.
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
    await page.goto("/gallery");

    await expect(page).toHaveURL(/\/login(?:\?|$)/);
    await expect(page.getByLabel("Email")).toBeVisible();
    await expect(page.locator("[data-gallery-root]")).toHaveCount(0);
  });
});
