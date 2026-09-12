import { test, expect } from "@playwright/test";
import { loginPageViaApi } from "./e2e-auth";
import { readE2EEnv } from "./e2e-env";
import { showScanStatus } from "../src/test/helpers/show-scan";

test("TV shows scan progress keeps the scan button busy and reports unmatched shows", async ({ page }) => {
  const env = readE2EEnv();
  await loginPageViaApi(page, env);
  let status = showScanStatus({ run_id: "", state: "idle", phase: "idle", total: 0 });
  let starts = 0;
  await page.route("**/api/settings", route => route.fulfill({ json: { error: false, data: { movies_dir: null, shows_dir: "/media/shows", music_dir: null } } }));
  await page.route("**/api/settings/scan/shows", route => {
    if (route.request().method() === "POST") {
      starts++;
      status = showScanStatus({ processed: 54, imported: 54, episodes: 60, active_files: ["Show.S01E01.mkv"] });
      return route.fulfill({ json: { error: false, message: "Show library scan started" } });
    }
    return route.fulfill({ json: { error: false, data: status } });
  });
  await page.goto("/settings/libraries");
  await expect(page.getByRole("status").filter({ hasText: "No TV shows scan has run yet." })).toBeVisible();
  const start = page.getByRole("button", { name: "Scan TV shows library", exact: true });
  await expect(start).toBeEnabled();
  await start.focus();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("button", { name: "Scanning TV shows library, please wait" })).toBeDisabled();
  await expect(page.getByRole("status").filter({ hasText: "Inspecting and importing episodes" })).toBeVisible();
  await expect(page.getByText("54 of 316 files processed · 54 imported · 0 updated · 0 unchanged")).toBeVisible();
  await expect(page.getByText("60 local episodes")).toBeVisible();
  await expect(page.getByText("Working on: Show.S01E01.mkv")).toBeVisible();
  await page.getByRole("tab", { name: "Account", exact: true }).click();
  await expect(page.getByText("Profile Information")).toBeVisible();
  await page.getByRole("tab", { name: "Libraries", exact: true }).click();
  await expect(page.getByRole("button", { name: "Scanning TV shows library, please wait" })).toBeDisabled();
  status = showScanStatus({ state: "completed-with-issues", phase: "enrichment", processed: 316, imported: 315, failed: 1, episodes: 348,
    enrichment_total: 5, enrichment_processed: 5, enriched: 3, enrichment_unmatched: 2, pending_enrichment: 6,
    issue_count: 2, issues: [
      { filename: "Unknown (2001)", phase: "enrichment", reason: "TMDB returned no matching show. Rename the folder or wait for a later scan; repeated misses back off." },
      { filename: "broken.mkv", phase: "local", reason: "The file could not be read or its name does not identify a season and episode. Existing records are preserved." },
    ] });
  await expect(page.getByRole("status").filter({ hasText: "TV shows scan completed with issues" })).toBeVisible();
  await expect(page.getByText("Descriptions: 5 of 5 attempted · 3 updated · 0 failed · 2 unmatched · 6 pending")).toBeVisible();
  await expect(page.getByText("348 local episodes")).toBeVisible();
  await expect(start).toBeEnabled();
  const summary = page.getByText("2 outstanding issues", { exact: true });
  await summary.focus();
  await page.keyboard.press("Enter");
  await expect(page.getByText("Unknown (2001): TMDB returned no matching show. Rename the folder or wait for a later scan; repeated misses back off.")).toBeVisible();
  expect(starts).toBe(1);
});
