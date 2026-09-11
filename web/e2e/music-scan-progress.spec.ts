import { test, expect } from "@playwright/test";
import { loginPageViaApi } from "./e2e-auth";
import { readE2EEnv } from "./e2e-env";
import { musicScanStatus } from "../src/test/helpers/music-scan";

test("music scan progress keeps the scan button busy and reports completion with issues", async ({ page }) => {
  const env = readE2EEnv();
  await loginPageViaApi(page, env);
  let status = musicScanStatus({ run_id: "", state: "idle", phase: "idle", total: 0 });
  let starts = 0;
  let statisticsRequests = 0;
  await page.route("**/api/music/stats", route => {
    statisticsRequests++;
    return route.continue();
  });
  await page.route("**/api/settings", route => route.fulfill({ json: { error: false, data: { movies_dir: null, shows_dir: null, music_dir: "/media/music" } } }));
  await page.route("**/api/settings/scan/music", route => {
    if (route.request().method() === "POST") {
      starts++;
      status = musicScanStatus({ total: 108, processed: 54, imported: 54, active_files: ["01 Intro.m4a"] });
      return route.fulfill({ json: { error: false, message: "Music library scan started" } });
    }
    return route.fulfill({ json: { error: false, data: status } });
  });
  await page.goto("/settings/libraries");
  await expect(page.getByRole("status").filter({ hasText: "No music scan has run yet." })).toBeVisible();
  const start = page.getByRole("button", { name: "Scan music library", exact: true });
  await expect(start).toBeEnabled();
  await start.focus();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("button", { name: "Scanning music library, please wait" })).toBeDisabled();
  await expect(page.getByRole("status").filter({ hasText: "Discovering and importing tracks" })).toBeVisible();
  await expect(page.getByText("54 of 108 files processed · 54 imported · 0 updated · 0 unchanged")).toBeVisible();
  await expect(page.getByText("Working on: 01 Intro.m4a")).toBeVisible();
  await page.getByRole("tab", { name: "Account", exact: true }).click();
  await expect(page.getByText("Profile Information")).toBeVisible();
  await page.getByRole("tab", { name: "Libraries", exact: true }).click();
  await expect(page.getByRole("button", { name: "Scanning music library, please wait" })).toBeDisabled();
  const beforeCompletion = statisticsRequests;
  status = musicScanStatus({ state: "completed-with-issues", phase: "enrichment", total: 108, processed: 108, imported: 107, failed: 1,
    enrichment_total: 3, enrichment_processed: 3, enriched: 40, enrichment_unmatched: 2,
    issue_count: 1, issues: [{ filename: "broken.flac", phase: "local", reason: "Unable to inspect, probe, or save this track." }] });
  await expect(page.getByRole("status").filter({ hasText: "Music scan completed with issues" })).toBeVisible();
  await expect(page.getByText("Spotify: 3 of 3 retried · 40 matched · 0 failed · 2 unmatched")).toBeVisible();
  await expect.poll(() => statisticsRequests).toBeGreaterThan(beforeCompletion);
  await expect(start).toBeEnabled();
  const summary = page.getByText("1 outstanding issues", { exact: true });
  await summary.focus();
  await page.keyboard.press("Enter");
  await expect(page.getByText("broken.flac: Unable to inspect, probe, or save this track.")).toBeVisible();
  expect(starts).toBe(1);
});
