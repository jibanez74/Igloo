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
// set is drawn by CI's pinned image, so a laptop runs the specs without
// comparing unless it asks to (VISUAL_STRICT=1), normally together with
// VISUAL_SNAPSHOT_DIR pointing at a set that laptop drew itself.
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
  reporter: [["list"], ["html", { open: "never" }]],
  // One canonical set: no platform or browser suffix in the file names.
  snapshotPathTemplate: `${snapshotDir}/{testFileName}/{arg}{ext}`,
  ignoreSnapshots: !comparesPixels,
  expect: {
    timeout: intEnv("E2E_EXPECT_TIMEOUT_MS", 30_000),
    toHaveScreenshot: {
      animations: "disabled",
      caret: "hide",
      scale: "css",
      // Both sides come from one renderer, so the comparison is exact: no
      // colour tolerance within a pixel and no pixel allowed to differ. The
      // default tolerance would let a border or surface shift through.
      threshold: 0,
      maxDiffPixels: 0,
    },
  },
  use: {
    ...devices["Desktop Chrome"],
    baseURL,
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 1,
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
