import { defineConfig, devices } from "@playwright/test";
import { intEnv } from "./e2e/e2e-env";

// Screenshot baselines of the component gallery and a few whole pages
// (docs/design-system.md §2.5). A separate config from playwright.config.ts:
// the gallery exists in the dev server only, so this run always serves the
// app with `bun run dev`, and its `*.visual.ts` files stay out of the
// production e2e run.

const webPort = intEnv("E2E_WEB_PORT", 3000);
const mockApiPort = intEnv("E2E_MOCK_API_PORT", 8080);
const baseURL = `http://127.0.0.1:${webPort}`;

// Pixels compare only against baselines the same renderer drew. The committed
// set is drawn in a pinned Playwright image by `make test-visual-docker`
// (which sets VISUAL_STRICT=1; CI no longer runs this config, but a run where
// CI is set still compares). Anywhere else the specs run without comparing unless asked to
// (VISUAL_STRICT=1), normally together with VISUAL_SNAPSHOT_DIR pointing at a
// set that machine drew.
const comparesPixels =
  Boolean(process.env.CI) || process.env.VISUAL_STRICT === "1";
const snapshotDir =
  process.env.VISUAL_SNAPSHOT_DIR ?? "{testDir}/__screenshots__";

export default defineConfig({
  testDir: "./e2e/visual",
  testMatch: "**/*.visual.ts",
  timeout: 60_000,
  forbidOnly: Boolean(process.env.CI),
  fullyParallel: false,
  // The mock API server is stateful, as in playwright.config.ts.
  workers: 1,
  // Chromium does not always rasterise one page to the same pixels: now and
  // then a load comes out with a few anti-aliased pixels of a rounded corner
  // one shade apart (about one test in a hundred). A real change differs on
  // every load, so a comparing run gets two more tries, on fresh pages, and
  // a test that needed one is reported as flaky rather than hidden.
  retries: comparesPixels ? 2 : 0,
  // The JSON report feeds the near-miss list `make test-visual-docker`
  // prints (e2e/visual/flake-summary.ts). test-results/ is emptied when a run
  // starts and the report is written when it ends.
  reporter: [
    ["list"],
    ["html", { open: "never" }],
    ["json", { outputFile: "test-results/visual-results.json" }],
  ],
  // One canonical set: no platform or browser suffix in the file names.
  snapshotPathTemplate: `${snapshotDir}/{testFileName}/{arg}{ext}`,
  ignoreSnapshots: !comparesPixels,
  expect: {
    timeout: intEnv("E2E_EXPECT_TIMEOUT_MS", 30_000),
    toHaveScreenshot: {
      animations: "disabled",
      caret: "hide",
      scale: "css",
      // Both sides come from one renderer, so nothing is tolerated: no
      // colour difference within a pixel and no pixel allowed to differ. The
      // default tolerance would let a surface shift through. Playwright's
      // comparator (pixelmatch) still does not count a differing pixel it
      // takes for anti-aliasing, mostly along text and curved edges, so the
      // specs assert through expectScreenshot (e2e/visual/visual-helpers.ts),
      // which follows this comparison with one that has no such allowance.
      threshold: 0,
      maxDiffPixels: 0,
    },
  },
  use: {
    ...devices["Desktop Chrome"],
    baseURL,
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 1,
    // Chromium normally repaints only the part of a raster tile that changed,
    // and the anti-aliased pixels along that part's edge then depend on what
    // was painted before it: the same page came out in two or three pictures
    // a shade apart on a rounded corner, by whether a transition had ended
    // when its neighbours were painted (measured on the movie genres page:
    // 15 loads in 60, none in 200 with this flag). Repainting whole tiles
    // makes the picture a function of the page alone.
    launchOptions: { args: ["--disable-partial-raster"] },
    trace: "retain-on-failure",
  },
  webServer: [
    {
      command: "bun ./e2e/mock-api-server.ts",
      url: `http://127.0.0.1:${mockApiPort}/health`,
      reuseExistingServer: false,
      timeout: 30_000,
      env: { E2E_MOCK_API_PORT: String(mockApiPort) },
    },
    {
      command: `bun run dev --host 127.0.0.1 --port ${webPort} --strictPort --open=false`,
      env: { E2E_MOCK_API_PORT: String(mockApiPort) },
      url: baseURL,
      reuseExistingServer: false,
      timeout: 60_000,
    },
  ],
});
