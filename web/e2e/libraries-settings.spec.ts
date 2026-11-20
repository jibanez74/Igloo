import { mkdtemp, mkdir, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, test } from "./e2e-test";
import type { Page } from "@playwright/test";

import type { SettingsType } from "../src/types";
import {
  fulfillJSON,
  getApiData,
  putApiData,
  readJSON,
} from "./e2e-api";
import { loginPageViaApi } from "./e2e-auth";
import { expectedRefusalRules } from "./e2e-browser-issues";
import { isRealInstance } from "./e2e-env";
import { auditResponsiveControls } from "./e2e-layout";

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

function fetchLibrarySettings(page: Page) {
  return getApiData<SettingsType>(page.context().request, "/api/settings");
}

// The save bar's status line: a live region with no role, so it is found by
// its exact text inside the tab, apart from the toasts outside it.
function formStatus(page: Page, text: string) {
  return page
    .getByRole("tabpanel", { name: "Libraries" })
    .getByText(text, { exact: true });
}

// Answers the scan triggers as the server does (settings_handler.go
// triggerScan), so the page's own wording is what the test reads.
const SCAN_ANSWERS: Record<string, { status: number; message: string }[]> = {
  // Already running, then started.
  "/api/settings/scan/movies": [
    { status: 409, message: "movie library scan is already in progress" },
    { status: 200, message: "Movie library scan started" },
  ],
  // A 5xx hides the server's message behind the page's fallback.
  "/api/settings/scan/music": [
    { status: 500, message: "music directory is not configured" },
    { status: 200, message: "Music library scan started" },
  ],
  // Queued behind a run of the library watcher's changes.
  "/api/settings/scan/shows": [{ status: 200, message: "Show library scan queued" }],
};

test.describe("Libraries settings", () => {
  test("manages library paths accessibly without console noise", async ({
    page,
    browserIssues,
  }) => {
    browserIssues.ignore(
      expectedRefusalRules(
        { method: "POST", path: "/api/settings/scan/movies", status: 409 },
        { method: "POST", path: "/api/settings/scan/music", status: 500 },
      ),
    );
    await loginPageViaApi(page);
    const baseline = await fetchLibrarySettings(page);
    const scanRequests: Record<string, number> = {};
    // Scan triggers only: the status poll (GET /api/settings/scan) is not
    // under scan/ and reaches the server, which reports idle. Stubbed on a
    // real instance too, so no scan runs.
    await page.route("**/api/settings/scan/**", async route => {
      const { pathname } = new URL(route.request().url());
      const answers = SCAN_ANSWERS[pathname];
      if (!answers) {
        await route.fallback();
        return;
      }
      const count = (scanRequests[pathname] = (scanRequests[pathname] ?? 0) + 1);
      const answer = answers[Math.min(count, answers.length) - 1];
      await fulfillJSON(
        route,
        { error: answer.status !== 200, message: answer.message },
        answer.status,
      );
    });

    let tempRoot: string | undefined;

    try {
      tempRoot = await mkdtemp(join(tmpdir(), "igloo-libraries-settings-"));
      const paths = {
        movies: join(tempRoot, "movies"),
        shows: join(tempRoot, "shows"),
        music: join(tempRoot, "music"),
      };
      await Promise.all(
        Object.values(paths).map(path => mkdir(path, { recursive: true })),
      );

      await page.goto("/settings/libraries");
      await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
      await expect(
        page.getByRole("heading", { name: "Library management" }),
      ).toBeVisible();

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
        await expect(input).toHaveAccessibleDescription(
          /directory path readable by the Igloo server/,
        );
      }

      await moviesInput.fill(paths.movies);
      await showsInput.fill(paths.shows);
      await musicInput.fill(paths.music);
      await expect(
        formStatus(page, "Library path changes are ready to save."),
      ).toBeVisible();
      await expect(
        page.getByRole("button", { name: "Save library paths" }),
      ).toBeEnabled();

      await moviesInput.focus();
      await page.keyboard.press("Tab");
      await expect(
        page.getByRole("button", { name: "Clear movies library path" }),
      ).toBeFocused();

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
      const saveBody = await readJSON<unknown>(saveResponse);
      expect(saveBody.error, saveBody.message).toBe(false);

      await expect(
        formStatus(page, "Library paths saved."),
      ).toBeVisible();

      await page.reload();
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
      await expect(
        page.getByRole("button", { name: "Scan TV shows library" }),
      ).toBeVisible();
      await expect(
        page.getByText("TV shows scanning unavailable"),
      ).toHaveCount(0);
      await expect(
        page.getByRole("status").filter({ hasText: "No movie scan has run yet." }),
      ).toBeVisible();
      await expect(
        page.getByRole("status").filter({ hasText: "No TV shows scan has run yet." }),
      ).toBeVisible();
      await expect(
        page.getByRole("status").filter({ hasText: "No music scan has run yet." }),
      ).toBeVisible();

      await page.getByRole("button", { name: "Clear music library path" }).click();
      await expect(musicInput).toHaveValue("");
      await expect(
        formStatus(page, "Library path changes are ready to save."),
      ).toBeVisible();
      await page.getByRole("button", { name: "Reset library paths" }).click();
      await expect(musicInput).toHaveValue(paths.music);

      const moviesScanButton = page.getByRole("button", {
        name: "Scan movies library",
      });

      await Promise.all([
        page.waitForResponse(response => {
          const url = new URL(response.url());
          return url.pathname === "/api/settings/scan/movies" && response.request().method() === "POST";
        }),
        moviesScanButton.click(),
      ]);
      await expect(
        formStatus(page, "movie library scan is already in progress"),
      ).toBeVisible();
      await expect(moviesScanButton).toBeEnabled();
      await expect(moviesScanButton).toHaveAttribute("aria-busy", "false");

      await Promise.all([
        page.waitForResponse(response => {
          const url = new URL(response.url());
          return url.pathname === "/api/settings/scan/movies" && response.request().method() === "POST";
        }),
        moviesScanButton.click(),
      ]);
      await expect(
        formStatus(page, "Movies library scan started."),
      ).toBeVisible();

      const musicScanButton = page.getByRole("button", { name: "Scan music library" });
      for (const shown of ["Failed to start music scan.", "Music library scan started."]) {
        await Promise.all([
          page.waitForResponse(response => {
            const url = new URL(response.url());
            return url.pathname === "/api/settings/scan/music" && response.request().method() === "POST";
          }),
          musicScanButton.click(),
        ]);
        await expect(formStatus(page, shown)).toBeVisible();
      }

      // The TV shows scan trigger is reachable by keyboard from its path input.
      const showsScanButton = page.getByRole("button", {
        name: "Scan TV shows library",
      });
      await showsInput.focus();
      await page.keyboard.press("Tab");
      await expect(
        page.getByRole("button", { name: "Clear TV shows library path" }),
      ).toBeFocused();
      await page.keyboard.press("Tab");
      await expect(showsScanButton).toBeFocused();
      await Promise.all([
        page.waitForResponse(response => {
          const url = new URL(response.url());
          return url.pathname === "/api/settings/scan/shows" && response.request().method() === "POST";
        }),
        page.keyboard.press("Enter"),
      ]);
      await expect(
        formStatus(
          page,
          "TV Shows library scan queued. It starts once the library watcher has applied its changes.",
        ),
      ).toBeVisible();
      await expect(showsScanButton).toBeEnabled();
      await expect(showsScanButton).toHaveAttribute("aria-busy", "false");

      await auditResponsiveControls(page, {
        ready: () => expect(page.getByRole("tabpanel", { name: "Libraries" })).toBeVisible(),
        requiredNames: requiredControlNames,
      });
    } finally {
      await page.unroute("**/api/settings/scan/**").catch(() => undefined);
      // Only a real instance keeps what the test saved; the mock resets before
      // each test, and a restore failing here would hide the test's own error.
      if (isRealInstance) {
        await putApiData(page.context().request, "/api/settings/libraries", baseline);
      }
      if (tempRoot) {
        await rm(tempRoot, { recursive: true, force: true });
      }
    }
  });

  test("keeps the edits and marks the path the server cannot read", async ({
    page,
    browserIssues,
  }) => {
    browserIssues.ignore(
      expectedRefusalRules({ method: "PUT", path: "/api/settings/libraries", status: 400 }),
    );
    await loginPageViaApi(page);
    const baseline = await fetchLibrarySettings(page);
    let tempRoot: string | undefined;

    try {
      tempRoot = await mkdtemp(join(tmpdir(), "igloo-libraries-rejected-"));
      const paths = {
        missing: join(tempRoot, "missing"),
        movies: join(tempRoot, "movies"),
        shows: join(tempRoot, "shows"),
        music: join(tempRoot, "music"),
      };
      await Promise.all(
        [paths.movies, paths.shows, paths.music].map(path => mkdir(path)),
      );

      await page.goto("/settings/libraries");
      const moviesInput = page.getByRole("textbox", { name: "Movies library path" });
      const showsInput = page.getByRole("textbox", { name: "TV shows library path" });
      const musicInput = page.getByRole("textbox", { name: "Music library path" });
      const save = page.getByRole("button", { name: "Save library paths" });
      await moviesInput.fill(paths.missing);
      await showsInput.fill(paths.shows);
      await musicInput.fill(paths.music);
      await save.click();

      // The server checks movies first and names it; the page marks that
      // field and keeps every edit for the correction.
      await expect(formStatus(page, "movies directory is not accessible")).toBeVisible();
      await expect(moviesInput).toHaveAttribute("aria-invalid", "true");
      await expect(showsInput).not.toHaveAttribute("aria-invalid", "true");
      await expect(moviesInput).toHaveValue(paths.missing);
      await expect(showsInput).toHaveValue(paths.shows);
      await expect(musicInput).toHaveValue(paths.music);
      expect(await fetchLibrarySettings(page)).toEqual(baseline);

      await moviesInput.fill(paths.movies);
      await expect(moviesInput).not.toHaveAttribute("aria-invalid", "true");
      await save.click();
      await expect(formStatus(page, "Library paths saved.")).toBeVisible();
      expect(await fetchLibrarySettings(page)).toEqual({
        movies_dir: paths.movies,
        shows_dir: paths.shows,
        music_dir: paths.music,
      });
    } finally {
      // Only a real instance keeps what the test saved.
      if (isRealInstance) {
        await putApiData(page.context().request, "/api/settings/libraries", baseline);
      }
      if (tempRoot) {
        await rm(tempRoot, { recursive: true, force: true });
      }
    }
  });
});
