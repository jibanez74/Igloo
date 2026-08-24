import { defineConfig, devices } from "@playwright/test";

function envInt(name: string, fallback: number) {
  const raw = process.env[name];
  if (!raw) return fallback;

  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

const hasExternalBaseURL = Boolean(process.env.E2E_BASE_URL);
const webPort = envInt("E2E_WEB_PORT", 3000);
const defaultBaseURL = `http://127.0.0.1:${webPort}`;

export default defineConfig({
  testDir: "./e2e",
  timeout: envInt("E2E_HLS_TEST_TIMEOUT_MS", 600_000),
  expect: {
    timeout: envInt("E2E_EXPECT_TIMEOUT_MS", 30_000),
  },
  fullyParallel: false,
  workers: 1,
  reporter: [["list"], ["html", { open: "never" }]],
  use: {
    baseURL: process.env.E2E_BASE_URL ?? defaultBaseURL,
    trace: "retain-on-failure",
    video: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  webServer: hasExternalBaseURL
    ? undefined
    : [
        {
          command: "bun ./e2e/mock-api-server.ts",
          url: "http://127.0.0.1:8080/health",
          reuseExistingServer: false,
          timeout: 30_000,
        },
        {
          command:
            `bun run dev --host 127.0.0.1 --port ${webPort} --strictPort --open=false`,
          url: defaultBaseURL,
          reuseExistingServer: false,
          timeout: 60_000,
        },
      ],
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
