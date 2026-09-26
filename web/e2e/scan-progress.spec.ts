import { expect, test } from "@playwright/test";
import { ALBUMS_PER_PAGE, MUSICIANS_PER_PAGE } from "../src/lib/constants";
import { movieScanStatus } from "../src/test/helpers/movie-scan";
import { musicScanStatus } from "../src/test/helpers/music-scan";
import { showScanStatus } from "../src/test/helpers/show-scan";
import { apiResponse, fulfillJSON, IDLE_SCAN } from "./e2e-api";
import { loginPageViaApi } from "./e2e-auth";
import { trackBrowserIssues } from "./e2e-browser-issues";
import { requireMockApi } from "./e2e-env";

// A scan started from Settings → Libraries and followed to its completion.
// The report's wording is unit-tested (src/test/settings/scan-progress.test.tsx);
// this covers what needs a browser: the keyboard start, the busy button
// surviving route changes, polling that carries on away from the settings
// page, the statistics refresh on completion, and the native <summary>
// disclosure of the issues.

type ScanKind = {
  name: string;
  scanPath: string;
  settings: Record<"movies_dir" | "shows_dir" | "music_dir", string | null>;
  idle: unknown;
  running: unknown;
  runningText: string;
  completed: unknown;
  completedText: string;
  issue: string;
  navLink: string;
  libraryHeading: string;
  statsPath: string;
  libraryPageStubs?: Record<string, unknown>;
};

const SCAN_KINDS: ScanKind[] = [
  {
    name: "movies",
    scanPath: "/api/settings/scan/movies",
    settings: { movies_dir: "/media/movies", shows_dir: null, music_dir: null },
    idle: movieScanStatus(IDLE_SCAN),
    running: movieScanStatus(),
    runningText: "Inspecting and importing movies",
    completed: movieScanStatus({
      state: "completed-with-issues", phase: "enrichment", processed: 418, imported: 417, failed: 1,
      issue_count: 1, issues: [{ filename: "copying.mkv", phase: "local", reason: "The file is still changing." }],
    }),
    completedText: "Movie scan completed with issues",
    issue: "copying.mkv: The file is still changing.",
    navLink: "Movies",
    libraryHeading: "Movie Library",
    statsPath: "/api/movies/stats",
  },
  {
    name: "music",
    scanPath: "/api/settings/scan/music",
    settings: { movies_dir: null, shows_dir: null, music_dir: "/media/music" },
    idle: musicScanStatus(IDLE_SCAN),
    running: musicScanStatus(),
    runningText: "Discovering and importing tracks",
    completed: musicScanStatus({
      state: "completed-with-issues", phase: "enrichment", processed: 2267, imported: 2266, failed: 1,
      issue_count: 1, issues: [{ filename: "broken.flac", phase: "local", reason: "Unable to inspect, probe, or save this track." }],
    }),
    completedText: "Music scan completed with issues",
    issue: "broken.flac: Unable to inspect, probe, or save this track.",
    navLink: "Music",
    libraryHeading: "Music Library",
    statsPath: "/api/music/stats",
    // The mock API server has no music library lists.
    libraryPageStubs: {
      "/api/music/albums": { albums: [], total: 0, page: 1, per_page: ALBUMS_PER_PAGE, total_pages: 0 },
      "/api/music/musicians": { musicians: [], total: 0, page: 1, per_page: MUSICIANS_PER_PAGE, total_pages: 0 },
      "/api/spotify/status": { available: false },
    },
  },
  {
    name: "TV shows",
    scanPath: "/api/settings/scan/shows",
    settings: { movies_dir: null, shows_dir: "/media/shows", music_dir: null },
    idle: showScanStatus(IDLE_SCAN),
    running: showScanStatus(),
    runningText: "Inspecting and importing episodes",
    completed: showScanStatus({
      state: "completed-with-issues", phase: "enrichment", processed: 316, imported: 315, failed: 1,
      issue_count: 1, issues: [{ filename: "Unknown (2001)", phase: "enrichment", reason: "TMDB returned no matching show." }],
    }),
    completedText: "TV shows scan completed with issues",
    issue: "Unknown (2001): TMDB returned no matching show.",
    navLink: "TV Shows",
    libraryHeading: "TV Show Library",
    statsPath: "/api/shows/stats",
  },
];

test.describe("Library scan progress", () => {
  requireMockApi();

  for (const kind of SCAN_KINDS) {
    test(`${kind.name} scan progress survives navigation and reports completion with issues`, async ({ page }) => {
      const browserIssues = trackBrowserIssues(page, {
        // The status endpoint goes down once on purpose, below.
        ignoreResponse: response =>
          response.status() === 503 && new URL(response.url()).pathname === kind.scanPath,
        ignoreConsole: (_type, text) =>
          text === "Failed to load resource: the server responded with a status of 503 (Service Unavailable)",
      });
      let status: unknown = kind.idle;
      let starts = 0;
      let unavailable = false;
      let completedServed = false;
      let completedAwayFromSettings = false;
      let statsRequestsAfterCompletion = 0;

      await loginPageViaApi(page);
      await page.route("**/api/settings", route => fulfillJSON(route, apiResponse(kind.settings)));
      await page.route(`**${kind.scanPath}`, async route => {
        if (route.request().method() === "POST") {
          starts++;
          status = kind.running;
          await fulfillJSON(route, { error: false, message: "Library scan started" });
          return;
        }
        if (unavailable) {
          await fulfillJSON(route, { error: true, message: "Status temporarily unavailable" }, 503);
          return;
        }
        // Flagged before the response leaves, so nothing it triggers can
        // arrive ahead of the flag.
        if (status === kind.completed) {
          completedServed = true;
          completedAwayFromSettings ||= !new URL(page.url()).pathname.startsWith("/settings");
        }
        await fulfillJSON(route, apiResponse(status));
      });
      // Only requests made after the page has been told the scan completed
      // count, so the library page's own load cannot pass for the refresh.
      await page.route(`**${kind.statsPath}`, async route => {
        if (completedServed) statsRequestsAfterCompletion++;
        await route.fallback();
      });
      for (const [path, data] of Object.entries(kind.libraryPageStubs ?? {})) {
        await page.route(`**${path}*`, route => fulfillJSON(route, apiResponse(data)));
      }

      await page.goto("/settings/libraries");
      const start = page.getByRole("button", { name: `Scan ${kind.name} library`, exact: true });
      const busy = page.getByRole("button", { name: `Scanning ${kind.name} library, please wait` });
      await expect(start).toBeEnabled();
      await start.focus();
      await page.keyboard.press("Enter");
      await expect(busy).toBeDisabled();
      await expect(page.getByRole("status").filter({ hasText: kind.runningText })).toBeVisible();

      await page.getByRole("tab", { name: "Account", exact: true }).click();
      await expect(page.getByText("Profile Information")).toBeVisible();
      await page.getByRole("tab", { name: "Libraries", exact: true }).click();
      await expect(busy).toBeDisabled();

      unavailable = true;
      await expect(page.getByRole("alert")).toContainText("Showing the last known progress");
      await expect(busy).toBeDisabled();
      unavailable = false;

      await page.getByRole("link", { name: kind.navLink, exact: true }).click();
      await expect(page.getByRole("heading", { level: 1, name: kind.libraryHeading })).toBeVisible();
      // A page reload (such as the dev server re-optimizing dependencies)
      // would refetch the statistics too; the marker proves the running app did.
      await page.evaluate(() => {
        (window as Window & { scanRefreshMarker?: boolean }).scanRefreshMarker = true;
      });
      status = kind.completed;
      await expect.poll(() => statsRequestsAfterCompletion).toBeGreaterThan(0);
      expect(
        await page.evaluate(
          () => (window as Window & { scanRefreshMarker?: boolean }).scanRefreshMarker,
        ),
      ).toBe(true);
      await expect.poll(() => completedAwayFromSettings).toBe(true);

      await page.getByRole("link", { name: "Settings", exact: true }).click();
      await page.getByRole("tab", { name: "Libraries", exact: true }).click();
      await expect(page.getByRole("status").filter({ hasText: kind.completedText })).toBeVisible();
      await expect(start).toBeEnabled();
      await page.getByText("1 outstanding issues", { exact: true }).focus();
      await page.keyboard.press("Enter");
      await expect(page.getByText(kind.issue)).toBeVisible();
      expect(starts).toBe(1);

      browserIssues.assertClean();
    });
  }
});
