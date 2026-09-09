import { mkdtemp, mkdir, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  expect,
  test,
  type APIResponse,
  type Locator,
  type Page,
} from "@playwright/test";

import { apiURL, readE2EEnv, type E2EEnv } from "./e2e-env";
import { trackBrowserIssues } from "./e2e-browser-issues";
import {
  readJSON,
} from "./e2e-api";
import { loginPageViaApi } from "./e2e-auth";

type LibrarySettings = {
  movies_dir: string | null;
  shows_dir: string | null;
  music_dir: string | null;
};

const responsiveViewports = [
  { name: "small phone", width: 360, height: 800 },
  { name: "phone", width: 390, height: 844 },
  { name: "tablet portrait", width: 768, height: 1024 },
  { name: "desktop", width: 1440, height: 900 },
];

const requiredControlNames = [
  "Movies library path",
  "TV shows library path",
  "Music library path",
  "Clear movies library path",
  "Clear TV shows library path",
  "Clear music library path",
  "Scan movies library",
  "Scan TV shows library",
  "Scan music library",
  "Reset library paths",
  "Save library paths",
];

async function expectAPIData<T>(response: APIResponse, expectedStatus: number) {
  expect(response.status()).toBe(expectedStatus);

  const body = await readJSON<T>(response);
  expect(body.error, body.message).toBe(false);
  expect(body.data).toBeTruthy();
  return body.data!;
}

async function fetchLibrarySettings(page: Page, env: E2EEnv) {
  const response = await page.context().request.get(apiURL(env, "/api/settings"), {
    failOnStatusCode: false,
  });

  return expectAPIData<LibrarySettings>(response, 200);
}

async function restoreLibrarySettings(
  page: Page,
  env: E2EEnv,
  settings: LibrarySettings,
) {
  const response = await page.context().request.put(
    apiURL(env, "/api/settings/libraries"),
    {
      data: settings,
      failOnStatusCode: false,
    },
  );
  expect(response.status()).toBe(200);

  const body = await readJSON<{ settings: LibrarySettings }>(response);
  expect(body.error, body.message).toBe(false);
}

async function expectDescriptionIncludes(
  page: Page,
  locator: Locator,
  expectedText: string,
) {
  const describedBy = await locator.getAttribute("aria-describedby");
  expect(describedBy).toBeTruthy();

  const descriptionText = await page.evaluate(ids => {
    return ids
      .split(/\s+/)
      .map(id => document.getElementById(id)?.textContent?.trim() ?? "")
      .filter(Boolean)
      .join(" ");
  }, describedBy ?? "");

  expect(descriptionText).toContain(expectedText);
}

async function activeElementName(page: Page) {
  return page.evaluate(() => {
    const active = document.activeElement;
    if (!(active instanceof HTMLElement)) {
      return null;
    }

    const ariaLabel = active.getAttribute("aria-label");
    if (ariaLabel) {
      return ariaLabel;
    }

    if (active.id) {
      const label = document.querySelector(
        `label[for="${CSS.escape(active.id)}"]`,
      );
      if (label?.textContent) {
        return label.textContent.trim();
      }
    }

    return (
      active.textContent?.trim() ||
      active.getAttribute("name") ||
      active.tagName
    );
  });
}

async function auditResponsiveLibrariesPage(page: Page) {
  for (const viewport of responsiveViewports) {
    await page.setViewportSize({
      width: viewport.width,
      height: viewport.height,
    });
    await expect(page.getByRole("tabpanel", { name: "Libraries" })).toBeVisible();

    const audit = await page.evaluate(requiredNames => {
      const isVisible = (element: Element) => {
        const style = window.getComputedStyle(element);
        const box = element.getBoundingClientRect();
        return (
          style.visibility !== "hidden" &&
          style.display !== "none" &&
          box.width > 0 &&
          box.height > 0
        );
      };

      const accessibleName = (element: Element) => {
        const labelledBy = (element.getAttribute("aria-labelledby") ?? "")
          .split(/\s+/)
          .filter(Boolean)
          .map(id => document.getElementById(id)?.textContent?.trim() ?? "")
          .filter(Boolean)
          .join(" ");

        if (labelledBy) {
          return labelledBy;
        }

        const ariaLabel = element.getAttribute("aria-label");
        if (ariaLabel) {
          return ariaLabel;
        }

        if (element.id) {
          const label = document.querySelector(
            `label[for="${CSS.escape(element.id)}"]`,
          );
          const labelText = label?.textContent?.trim();
          if (labelText) {
            return labelText;
          }
        }

        return element.textContent?.trim() ?? "";
      };

      const interactive = Array.from(
        document.querySelectorAll(
          'button, a[href], input, select, textarea, [role="button"], [tabindex]:not([tabindex="-1"])',
        ),
      ).filter(isVisible);

      const unlabeled = interactive
        .filter(element => !accessibleName(element))
        .map(element => ({
          tag: element.tagName.toLowerCase(),
          text: element.textContent?.trim().slice(0, 80) ?? "",
        }));

      const names = interactive.map(accessibleName);
      const missing = requiredNames.filter(name => !names.includes(name));

      const overflowX = Array.from(document.querySelectorAll("body, body *"))
        .filter(isVisible)
        .filter(element => {
          const rect = element.getBoundingClientRect();
          return rect.left < -1 || rect.right > window.innerWidth + 1;
        })
        .map(element => ({
          tag: element.tagName.toLowerCase(),
          text: element.textContent?.trim().slice(0, 80) ?? "",
          right: Math.round(element.getBoundingClientRect().right),
        }));

      return {
        viewportWidth: window.innerWidth,
        scrollWidth: document.documentElement.scrollWidth,
        missing,
        unlabeled,
        overflowX,
      };
    }, requiredControlNames);

    expect(
      audit.scrollWidth,
      `${viewport.name} should not create horizontal page overflow`,
    ).toBeLessThanOrEqual(audit.viewportWidth + 1);
    expect(audit.missing, `${viewport.name} missing controls`).toEqual([]);
    expect(audit.unlabeled, `${viewport.name} unlabeled controls`).toEqual([]);
    expect(audit.overflowX, `${viewport.name} overflow elements`).toEqual([]);
  }
}

test.describe("Libraries settings", () => {
  test("manages library paths accessibly without console noise", async ({
    page,
  }) => {
    const env = readE2EEnv();
    const tempRoot = await mkdtemp(join(tmpdir(), "igloo-libraries-settings-"));
    const paths = {
      movies: join(tempRoot, "movies"),
      shows: join(tempRoot, "shows"),
      music: join(tempRoot, "music"),
    };
    await Promise.all(
      Object.values(paths).map(path => mkdir(path, { recursive: true })),
    );

    await loginPageViaApi(page, env);
    const baseline = await fetchLibrarySettings(page, env);
    const tracker = trackBrowserIssues(page);

    let movieScanRequests = 0;
    let showScanRequests = 0;
    let scanRequests = 0;
    let releaseShowScan = () => {};
    const showScanGate = new Promise<void>(resolve => { releaseShowScan = resolve; });
    await page.route("**/api/settings/scan/**", async route => {
      const url = new URL(route.request().url());
      expect(route.request().method()).toBe("POST");
      scanRequests += 1;
      if (url.pathname === "/api/settings/scan/shows") {
        showScanRequests += 1;
        await showScanGate;
      }
      if (url.pathname === "/api/settings/scan/movies") {
        movieScanRequests += 1;
      }

      if (url.pathname === "/api/settings/scan/movies" && movieScanRequests === 1) {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            error: true,
            message: "Failed to start movies scan.",
          }),
        });
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          error: false,
          message: "Library scan started",
        }),
      });
    });

    try {
      await restoreLibrarySettings(page, env, { ...baseline, shows_dir: null });
      await page.goto(apiURL(env, "/settings/libraries"), {
        waitUntil: "networkidle",
      });
      await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
      await expect(page.getByText("Library Management")).toBeVisible();

      const moviesInput = page.getByRole("textbox", {
        name: "Movies library path",
      });
      const showsInput = page.getByRole("textbox", {
        name: "TV shows library path",
      });
      const musicInput = page.getByRole("textbox", {
        name: "Music library path",
      });

      for (const input of [moviesInput, showsInput, musicInput]) {
        await expectDescriptionIncludes(
          page,
          input,
          "directory path readable by the Igloo server",
        );
      }

      const showsScanButton = page.getByRole("button", { name: "Scan TV shows library", exact: true });
      await expect(showsScanButton).toHaveCount(0);
      await moviesInput.fill(paths.movies);
      await showsInput.fill(paths.shows);
      await musicInput.fill(paths.music);
      await expect(showsScanButton).toHaveCount(0);
      await expect(
        page.locator('p[aria-live="polite"]').filter({
          hasText: "Library path changes are ready to save.",
        }),
      ).toBeVisible();
      await expect(
        page.getByRole("button", { name: "Save library paths" }),
      ).toBeEnabled();

      await moviesInput.focus();
      await page.keyboard.press("Tab");
      await expect.poll(() => activeElementName(page)).toBe(
        "Clear movies library path",
      );

      const saveResponsePromise = page.waitForResponse(response => {
        const url = new URL(response.url());
        return (
          url.pathname === "/api/settings/libraries" &&
          response.request().method() === "PUT"
        );
      });
      await page.getByRole("button", { name: "Save library paths" }).click();
      const saveResponse = await saveResponsePromise;
      expect(saveResponse.status()).toBe(200);
      const saveBody = await readJSON<{ settings: LibrarySettings }>(
        saveResponse,
      );
      expect(saveBody.error, saveBody.message).toBe(false);

      await expect(
        page.locator('p[aria-live="polite"]').filter({
          hasText: "Library paths saved.",
        }),
      ).toBeVisible();

      expect(scanRequests).toBe(0);
      await expect(showsScanButton).toBeEnabled();
      await page.reload({ waitUntil: "networkidle" });
      await expect(moviesInput).toHaveValue(paths.movies);
      await expect(showsInput).toHaveValue(paths.shows);
      await expect(musicInput).toHaveValue(paths.music);
      await expect(
        page.getByRole("button", { name: "Clear movies library path" }),
      ).toBeVisible();
      await expect(
        page.getByRole("button", { name: "Clear TV shows library path" }),
      ).toBeVisible();
      await expect(
        page.getByRole("button", { name: "Clear music library path" }),
      ).toBeVisible();
      await expect(
        page.getByRole("button", { name: "Scan movies library" }),
      ).toBeVisible();
      await expect(
        page.getByRole("button", { name: "Scan music library" }),
      ).toBeVisible();
      await expect(showsScanButton).toBeEnabled();
      await showsInput.fill(`${paths.shows}/unsaved`);
      await expect(showsScanButton).toBeDisabled();
      await page.getByRole("button", { name: "Reset library paths" }).click();
      await expect(showsInput).toHaveValue(paths.shows);
      await expect(showsScanButton).toBeEnabled();

      let releaseSave = () => {};
      const saveGate = new Promise<void>(resolve => { releaseSave = resolve; });
      await page.route("**/api/settings/libraries", async route => {
        await saveGate;
        await route.continue();
      });
      await musicInput.fill("");
      await page.getByRole("button", { name: "Save library paths" }).click();
      await expect(showsScanButton).toBeDisabled();
      releaseSave();
      await expect(showsScanButton).toBeEnabled();
      await page.unroute("**/api/settings/libraries");
      await musicInput.fill(paths.music);
      await page.getByRole("button", { name: "Save library paths" }).click();
      await expect(showsScanButton).toBeEnabled();
      expect(scanRequests).toBe(0);

      await showsInput.focus();
      await page.keyboard.press("Tab");
      await expect.poll(() => activeElementName(page)).toBe("Clear TV shows library path");
      await page.keyboard.press("Tab");
      await expect(showsScanButton).toBeFocused();
      await page.keyboard.press("Enter");
      const pendingShowScan = page.getByRole("button", { name: "Scanning TV shows library, please wait" });
      await expect(pendingShowScan).toBeDisabled();
      await expect(pendingShowScan).toHaveAttribute("aria-busy", "true");
      await expect(page.locator('p[aria-live="polite"]')).toHaveText("Starting TV shows library scan...");
      await expect(page.getByRole("button", { name: "Scan movies library" })).toBeDisabled();
      await expect(page.getByRole("button", { name: "Scan music library" })).toBeDisabled();
      await pendingShowScan.evaluate(button => {
        if (button instanceof HTMLButtonElement) {
          button.click();
          button.click();
        }
      });
      await page.keyboard.press("Enter");
      await expect.poll(() => showScanRequests).toBe(1);
      releaseShowScan();
      await expect(page.locator('p[aria-live="polite"]')).toHaveText("TV shows library scan started.");
      await expect(page.getByText("TV shows library scan has been initiated", { exact: true })).toBeVisible();
      await expect(showsScanButton).toBeEnabled();
      await expect(showsScanButton).toHaveAttribute("aria-busy", "false");
      expect(showScanRequests).toBe(1);

      await page.getByRole("button", { name: "Clear music library path" }).click();
      await expect(musicInput).toHaveValue("");
      await expect(
        page.locator('p[aria-live="polite"]').filter({
          hasText: "Library path changes are ready to save.",
        }),
      ).toBeVisible();
      await page.getByRole("button", { name: "Reset library paths" }).click();
      await expect(musicInput).toHaveValue(paths.music);

      const moviesScanButton = page.getByRole("button", {
        name: "Scan movies library",
      });

      await Promise.all([
        page.waitForResponse(response => {
          const url = new URL(response.url());
          return url.pathname === "/api/settings/scan/movies";
        }),
        moviesScanButton.click(),
      ]);
      await expect(
        page.locator('p[aria-live="polite"]').filter({
          hasText: "Failed to start movies scan.",
        }),
      ).toBeVisible();
      await expect(moviesScanButton).toBeEnabled();
      await expect(moviesScanButton).toHaveAttribute("aria-busy", "false");

      await Promise.all([
        page.waitForResponse(response => {
          const url = new URL(response.url());
          return url.pathname === "/api/settings/scan/movies";
        }),
        moviesScanButton.click(),
      ]);
      await expect(
        page.locator('p[aria-live="polite"]').filter({
          hasText: "Movies library scan started.",
        }),
      ).toBeVisible();

      await Promise.all([
        page.waitForResponse(response => {
          const url = new URL(response.url());
          return url.pathname === "/api/settings/scan/music";
        }),
        page.getByRole("button", { name: "Scan music library" }).click(),
      ]);
      await expect(
        page.locator('p[aria-live="polite"]').filter({
          hasText: "Music library scan started.",
        }),
      ).toBeVisible();

      await auditResponsiveLibrariesPage(page);

      await page.getByRole("button", { name: "Clear TV shows library path" }).click();
      await expect(showsInput).toHaveValue("");
      await expect(showsScanButton).toBeDisabled();
      const scansBeforeClear = scanRequests;
      await page.getByRole("button", { name: "Save library paths" }).click();
      await expect(showsScanButton).toHaveCount(0);
      expect((await fetchLibrarySettings(page, env)).shows_dir).toBeNull();
      await page.reload({ waitUntil: "networkidle" });
      await expect(showsInput).toHaveValue("");
      await expect(showsScanButton).toHaveCount(0);
      expect(scanRequests).toBe(scansBeforeClear);
    } finally {
      releaseShowScan();
      await page.unroute("**/api/settings/scan/**").catch(() => undefined);
      await restoreLibrarySettings(page, env, baseline);
      await rm(tempRoot, { recursive: true, force: true });
    }

    tracker.assertClean();
  });

  for (const failure of [
    { status: 409, message: "show library scan is already in progress" },
    { status: 500, message: "shows directory is not configured" },
  ]) {
    test(`reports TV scan ${failure.status} errors and allows retry`, async ({ page }) => {
      const env = readE2EEnv();
      await loginPageViaApi(page, env);
      await page.route("**/api/settings", route => route.fulfill({
        json: { error: false, data: { movies_dir: null, shows_dir: "/media/shows", music_dir: null } },
      }));
      let requests = 0;
      await page.route("**/api/settings/scan/shows", route => {
        expect(route.request().method()).toBe("POST");
        requests += 1;
        return route.fulfill(requests === 1
          ? { status: failure.status, json: { error: true, message: failure.message } }
          : { status: 200, json: { error: false, message: "Show library scan started" } });
      });
      await page.goto(apiURL(env, "/settings/libraries"));
      const scanButton = page.getByRole("button", { name: "Scan TV shows library", exact: true });
      await scanButton.click();
      await expect(page.locator('p[aria-live="polite"]')).toHaveText(failure.message);
      await expect(page.locator('[data-sonner-toast]')).toContainText(failure.message);
      await expect(scanButton).toBeEnabled();
      await expect(scanButton).toHaveAttribute("aria-busy", "false");
      await scanButton.click();
      await expect(page.locator('p[aria-live="polite"]')).toHaveText("TV shows library scan started.");
      expect(requests).toBe(2);
    });
  }

});
