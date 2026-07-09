import { expect, test, type APIRequestContext, type Page } from "@playwright/test";

import { readE2EEnv } from "./e2e-env";

type ApiResponse<T> = {
  error: boolean;
  message?: string;
  data?: T;
};

type InitiateData = {
  code: string;
  secret: string;
  expires_in_seconds: number;
  poll_interval_seconds: number;
};

type RedeemData = {
  status: "pending" | "approved";
  token?: string;
  device?: { id: number; name: string };
};

const env = readE2EEnv();
const deviceName = `E2E Quick Connect ${Date.now().toString(36)}`;

async function login(page: Page) {
  const response = await page.request.post("/api/auth/login", {
    data: { email: env.email, password: env.password },
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(200);
}

async function initiate(request: APIRequestContext): Promise<InitiateData> {
  const response = await request.post("/api/quick-connect/initiate", {
    data: { device_name: deviceName, platform: "android_tv", app_version: "e2e" },
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(201);

  const body = (await response.json()) as ApiResponse<InitiateData>;
  expect(body.error).toBe(false);
  expect(body.data?.code).toBeTruthy();
  expect(body.data?.secret).toBeTruthy();
  return body.data!;
}

async function redeem(
  request: APIRequestContext,
  code: string,
  secret: string,
): Promise<RedeemData> {
  const response = await request.post("/api/quick-connect/redeem", {
    data: { code, secret },
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(200);

  const body = (await response.json()) as ApiResponse<RedeemData>;
  expect(body.error).toBe(false);
  return body.data!;
}

// Runs against a real Igloo instance only: the pairing flow needs the actual
// in-memory quick-connect broker and devices table.
test.describe("Quick Connect pairing", () => {
  test.skip(
    !process.env.E2E_BASE_URL,
    "Set E2E_BASE_URL (plus E2E_ADMIN_EMAIL / E2E_ADMIN_PASSWORD) to run the quick-connect e2e test.",
  );

  test("pairs a device end-to-end and revokes it", async ({ page, request }) => {
    // The "device" (unauthenticated request context) starts pairing.
    const { code, secret } = await initiate(request);

    const pending = await redeem(request, code, secret);
    expect(pending.status).toBe("pending");

    // The user approves the code from account settings.
    await login(page);
    await page.goto("/settings/account");

    const codeInput = page.getByRole("textbox", { name: "Quick Connect code" });
    await codeInput.fill(code);
    await page.getByRole("button", { name: "Approve device" }).click();

    // The device polls again and receives its token exactly once.
    let token = "";
    await expect
      .poll(async () => {
        const result = await redeem(request, code, secret);
        if (result.status === "approved" && result.token) {
          token = result.token;
        }
        return result.status;
      }, { timeout: 10_000 })
      .toBe("approved");
    expect(token).toMatch(/^igd_/);

    // The token authenticates API requests.
    const me = await request.get("/api/auth/user", {
      headers: { Authorization: `Bearer ${token}` },
      failOnStatusCode: false,
    });
    expect(me.status()).toBe(200);

    // The paired device shows up in the list (the card refetches a few
    // seconds after approval, once the device has redeemed its code).
    await expect(page.locator("li", { hasText: deviceName })).toBeVisible({
      timeout: 15_000,
    });

    // Revoke it from the UI.
    await page.getByRole("button", { name: `Revoke ${deviceName}` }).click();
    await page
      .getByRole("alertdialog")
      .getByRole("button", { name: "Revoke" })
      .click();
    await expect(page.locator("li", { hasText: deviceName })).toHaveCount(0);

    // The revoked token no longer authenticates.
    const revoked = await request.get("/api/auth/user", {
      headers: { Authorization: `Bearer ${token}` },
      failOnStatusCode: false,
    });
    expect(revoked.status()).toBe(401);
  });
});
