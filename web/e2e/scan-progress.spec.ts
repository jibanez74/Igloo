import { expect, test } from "./e2e-test";
import { ALBUMS_PER_PAGE, MUSICIANS_PER_PAGE } from "../src/lib/constants";
import {
  movieScanStatus,
  musicScanStatus,
  scanStatuses,
  showScanStatus,
} from "../src/test/helpers/scan-status";
import { apiResponse, fulfillJSON, SCAN_STATUS_PATH } from "./e2e-api";
import type { ScanStatuses } from "../src/types/settings";
import { loginPageViaApi } from "./e2e-auth";
import { expectedRefusalRules } from "./e2e-browser-issues";
import { requireMockApi } from "./e2e-env";

// A scan started from Settings → Libraries and followed to its completion.
// The report's wording is unit-tested (src/test/settings/scan-progress.test.tsx);
// this covers what needs a browser: the keyboard start, the busy button
// surviving route changes, polling that carries on away from the settings
// page, the statistics refresh on completion, and the native <summary>
// disclosure of the issues.

type ScanKind = {
  name: string;
  triggerPath: string;
  settings: Record<"movies_dir" | "shows_dir" | "music_dir", string | null>;
  running: ScanStatuses;
  runningText: string;
  completed: ScanStatuses;
  completedText: string;
  unavailableText: string;
  issue: string;
  navLink: string;
  libraryHeading: string;
  statsPath: string;
  libraryPageStubs?: Record<string, unknown>;
};

const SCAN_KINDS: ScanKind[] = [
  {
    name: "movies",
    triggerPath: "/api/settings/scan/movies",
    settings: { movies_dir: "/media/movies", shows_dir: null, music_dir: null },
    running: scanStatuses({ movies: movieScanStatus() }),
    runningText: "Inspecting and importing movies",
    completed: scanStatuses({ movies: movieScanStatus({
      state: "completed-with-issues", phase: "enrichment", processed: 418, imported: 417, failed: 1,
      issue_count: 1, issues: [{ filename: "copying.mkv", phase: "local", reason: "The file is still changing." }],
    }) }),
    completedText: "Movie scan completed with issues",
    unavailableText: "Movie scan status is unavailable",
    issue: "copying.mkv: The file is still changing.",
    navLink: "Movies",
    libraryHeading: "Movie Library",
    statsPath: "/api/movies/stats",
  },
  {
    name: "music",
    triggerPath: "/api/settings/scan/music",
    settings: { movies_dir: null, shows_dir: null, music_dir: "/media/music" },
    running: scanStatuses({ music: musicScanStatus() }),
    runningText: "Discovering and importing tracks",
    completed: scanStatuses({ music: musicScanStatus({
      state: "completed-with-issues", phase: "enrichment", processed: 2267, imported: 2266, failed: 1,
      issue_count: 1, issues: [{ filename: "broken.flac", phase: "local", reason: "Unable to inspect, probe, or save this track." }],
    }) }),
    completedText: "Music scan completed with issues",
    unavailableText: "Music scan status is unavailable",
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
    triggerPath: "/api/settings/scan/shows",
    settings: { movies_dir: null, shows_dir: "/media/shows", music_dir: null },
    running: scanStatuses({ shows: showScanStatus() }),
    runningText: "Inspecting and importing episodes",
    completed: scanStatuses({ shows: showScanStatus({
      state: "completed-with-issues", phase: "enrichment", processed: 316, imported: 315, failed: 1,
      issue_count: 1, issues: [{ filename: "Unknown (2001)", phase: "enrichment", reason: "TMDB returned no matching show." }],
    }) }),
    completedText: "TV shows scan completed with issues",
    unavailableText: "TV shows scan status is unavailable",
    issue: "Unknown (2001): TMDB returned no matching show.",
    navLink: "TV Shows",
    libraryHeading: "TV Show Library",
    statsPath: "/api/shows/stats",
  },
];

test.describe("Library scan progress", () => {
  requireMockApi();

  for (const kind of SCAN_KINDS) {
    test(`${kind.name} scan progress survives navigation and reports completion with issues`, async ({ page, browserIssues }) => {
      // The status endpoint goes down once on purpose, below.
      browserIssues.ignore(expectedRefusalRules({ method: "GET", path: SCAN_STATUS_PATH, status: 503 }));
      let status = scanStatuses();
      let starts = 0;
      let unavailable = false;
      let completedServed = false;
      let completedAwayFromSettings = false;
      let statsRequestsAfterCompletion = 0;

      await loginPageViaApi(page);
      await page.route("**/api/settings", route => fulfillJSON(route, apiResponse(kind.settings)));
      await page.route(`**${kind.triggerPath}`, async route => {
        if (route.request().method() !== "POST") {
          await route.fallback();
          return;
        }
        starts++;
        status = kind.running;
        await fulfillJSON(route, { error: false, message: "Library scan started" });
      });
      // The report of the other two libraries stays idle.
      await page.route(`**${SCAN_STATUS_PATH}`, async route => {
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
      const busy = page.getByRole("button", { name: `Scanning ${kind.name} library...` });
      await expect(start).toBeEnabled();
      await start.focus();
      await page.keyboard.press("Enter");
      await expect(busy).toBeDisabled();
      await expect(page.getByRole("status").filter({ hasText: kind.runningText })).toBeVisible();

      await page.getByRole("tab", { name: "Account", exact: true }).click();
      await expect(page.getByText("Profile information")).toBeVisible();
      await page.getByRole("tab", { name: "Libraries", exact: true }).click();
      await expect(busy).toBeDisabled();

      // One request reports every library, so its failure leaves each
      // library on its last known report.
      unavailable = true;
      await expect(page.getByRole("alert").filter({ hasText: kind.unavailableText })).toContainText(
        "Showing the last known progress",
      );
      await expect(page.getByRole("alert").filter({ hasText: "Showing the last known progress" })).toHaveCount(3);
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
      // The issue list stays folded until its <summary> is opened.
      await expect(page.getByText(kind.issue)).toBeHidden();
      await page.getByText("1 outstanding issue", { exact: true }).focus();
      await page.keyboard.press("Enter");
      await expect(page.getByText(kind.issue)).toBeVisible();
      expect(starts).toBe(1);
    });
  }

  // How a run ends without completing. One library is enough: the loop above
  // shows the three share the report's handling.
  for (const outcome of [
    {
      state: "failed",
      text: "Movie scan stopped after an error. Committed movies are available.",
    },
    {
      state: "canceled",
      text: "Movie scan canceled. Committed movies are available.",
    },
  ] as const) {
    test(`a movies scan that ends ${outcome.state} frees the button and refreshes the library`, async ({ page }) => {
      let status = scanStatuses();
      let endedServed = false;
      let statsRequestsAfterEnd = 0;
      const ended = scanStatuses({
        movies: movieScanStatus({ state: outcome.state, processed: 120, imported: 120 }),
      });

      await loginPageViaApi(page);
      await page.route("**/api/settings", route =>
        fulfillJSON(route, apiResponse({ movies_dir: "/media/movies", shows_dir: null, music_dir: null })),
      );
      await page.route("**/api/settings/scan/movies", async route => {
        status = scanStatuses({ movies: movieScanStatus() });
        await fulfillJSON(route, { error: false, message: "Movie library scan started" });
      });
      await page.route(`**${SCAN_STATUS_PATH}`, async route => {
        if (status === ended) endedServed = true;
        await fulfillJSON(route, apiResponse(status));
      });
      await page.route("**/api/movies/stats", async route => {
        if (endedServed) statsRequestsAfterEnd++;
        await route.fallback();
      });

      await page.goto("/settings/libraries");
      const start = page.getByRole("button", { name: "Scan movies library", exact: true });
      await start.click();
      await expect(page.getByRole("button", { name: "Scanning movies library..." })).toBeDisabled();
      await expect(page.getByRole("status").filter({ hasText: "Inspecting and importing movies" })).toBeVisible();

      status = ended;
      await expect(page.getByRole("status").filter({ hasText: outcome.text })).toBeVisible();
      await expect(start).toBeEnabled();
      // The run's end refreshes the library's queries, statistics included.
      await expect.poll(() => statsRequestsAfterEnd).toBeGreaterThan(0);
    });
  }
});
