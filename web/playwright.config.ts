import { defineConfig, devices } from "@playwright/test";

function envInt(name: string, fallback: number) {
  const raw = process.env[name];
  if (!raw) return fallback;

  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

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
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3000",
    trace: "retain-on-failure",
    video: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
