import { test, expect } from "@playwright/test";
import { loginPageViaApi } from "./e2e-auth";
import { readE2EEnv } from "./e2e-env";
import { movieScanStatus } from "../src/test/helpers/movie-scan";

test("movie scan progress survives settings navigation and reports partial completion", async ({ page }) => {
  const env = readE2EEnv();
  await loginPageViaApi(page, env);
  let status = movieScanStatus({ run_id: "", state: "idle", phase: "idle", total: 0 });
  let starts = 0;
  let unavailable = false;
  let statisticsRequests = 0;
  let completedAwayFromSettings = false;
  await page.route("**/api/movies/stats", route => {
    statisticsRequests++;
    return route.continue();
  });
  await page.route("**/api/settings", route => route.fulfill({ json: { error: false, data: { movies_dir: "/media/movies", shows_dir: null, music_dir: null } } }));
  await page.route("**/api/settings/scan/movies", route => {
    if (route.request().method() === "POST") {
      starts++;
      status = movieScanStatus({ active_files: ["first.mkv", "second.mkv"] });
      return route.fulfill({ json: { error: false, message: "Movie library scan started" } });
    }
    if (unavailable) return route.fulfill({ status: 503, json: { error: true, message: "Status temporarily unavailable" } });
    if (status.state === "completed-with-issues" && new URL(page.url()).pathname === "/movies") completedAwayFromSettings = true;
    return route.fulfill({ json: { error: false, data: status } });
  });
  await page.goto("/settings/libraries");
  const start = page.getByRole("button", { name: "Scan movies library", exact: true });
  await expect(start).toBeEnabled();
  await start.focus();
  await page.keyboard.press("Enter");
  await expect(page.getByRole("button", { name: "Scanning movies library, please wait" })).toBeDisabled();
  await expect(page.getByRole("status").filter({ hasText: "Inspecting and importing movies" })).toBeVisible();
  await page.getByRole("tab", { name: "Account", exact: true }).click();
  await expect(page.getByText("Profile Information")).toBeVisible();
  await page.getByRole("tab", { name: "Libraries", exact: true }).click();
  await expect(page.getByRole("button", { name: "Scanning movies library, please wait" })).toBeDisabled();
  unavailable = true;
  await expect(page.getByRole("alert")).toContainText("Showing the last known progress");
  await expect(page.getByRole("button", { name: "Scanning movies library, please wait" })).toBeDisabled();
  unavailable = false;
  await page.getByRole("link", { name: "Movies", exact: true }).click();
  await expect(page).toHaveURL(/\/movies\?/);
  const beforeCompletion = statisticsRequests;
  status = movieScanStatus({ state: "completed-with-issues", phase: "enrichment", processed: 418, imported: 416, failed: 1, deferred: 1,
    issue_count: 1, issues: [{ filename: "copying.mkv", phase: "local", reason: "The file is still changing." }] });
  await expect.poll(() => completedAwayFromSettings).toBe(true);
  await expect.poll(() => statisticsRequests).toBeGreaterThan(beforeCompletion);
  await page.getByRole("link", { name: "Settings", exact: true }).click();
  await page.getByRole("tab", { name: "Libraries", exact: true }).click();
  await expect(page.getByRole("status").filter({ hasText: "Movie scan completed with issues" })).toBeVisible();
  await expect(start).toBeEnabled();
  const summary = page.getByText("1 outstanding issues", { exact: true });
  await summary.focus();
  await page.keyboard.press("Enter");
  await expect(page.getByText("copying.mkv: The file is still changing.")).toBeVisible();
  expect(starts).toBe(1);
});
