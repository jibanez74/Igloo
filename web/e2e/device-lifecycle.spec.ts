import { expect, test, type APIRequestContext } from "@playwright/test";

import { readE2EEnv } from "./e2e-env";
import type { ApiResponse } from "./e2e-api";
import { loginPageViaApi } from "./e2e-auth";

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

type DevicesData = {
  devices: Array<{ id: number; name: string }>;
};

const env = readE2EEnv();
const deviceName = "Mock Living Room TV";
const renamedDeviceName = "Mock Bedroom TV";

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

// Runs against the mock API server only: the real-server flavor of this flow
// lives in quick-connect.spec.ts, gated behind E2E_BASE_URL.
test.describe("Device lifecycle (mocked)", () => {
  test.skip(
    Boolean(process.env.E2E_BASE_URL),
    "Covered by quick-connect.spec.ts against a live server.",
  );

  test.beforeEach(async ({ page }) => {
    // Mock device state persists across specs in a run; clean up leftovers.
    await loginPageViaApi(page, env, { assertBody: false });
    const response = await page.request.get("/api/devices", {
      failOnStatusCode: false,
    });
    expect(response.status()).toBe(200);
    const body = (await response.json()) as ApiResponse<DevicesData>;
    for (const device of body.data?.devices ?? []) {
      await page.request.delete(`/api/devices/${device.id}`, {
        failOnStatusCode: false,
      });
    }
  });

  test("pairs, renames, and revokes a device through the settings UI", async ({
    page,
    request,
  }) => {
    // The "device" (unauthenticated request context) starts pairing.
    const { code, secret } = await initiate(request);

    const pending = await redeem(request, code, secret);
    expect(pending.status).toBe("pending");

    // The user enters the code and is shown which device is asking before
    // anything is approved.
    await page.goto("/settings/account");
    await page.getByRole("textbox", { name: "Quick Connect code" }).fill(code);
    await page.getByRole("button", { name: "Continue" }).click();
    await expect(
      page.getByRole("heading", { name: "Approve this device?" }),
    ).toBeVisible();
    await expect(page.getByText(deviceName)).toBeVisible();
    await page.getByRole("button", { name: "Approve device" }).click();

    // While the device finishes signing in, the card shows a waiting status.
    await expect(page.getByText(/Waiting for/)).toBeVisible();

    // The device polls again and receives its token.
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

    // The card polls the devices list every 2 seconds while waiting, so the
    // new device shows up in the list shortly after it redeems its code.
    // Exclude Sonner toasts: the "Device connected" toast is also a list
    // item containing the device name.
    const deviceItem = page.locator("li:not([data-sonner-toast])", {
      hasText: deviceName,
    });
    await expect(deviceItem).toBeVisible({ timeout: 15_000 });

    // Rename it inline (scope Save to the row — the page has other Save buttons).
    await page.getByRole("button", { name: `Rename ${deviceName}` }).click();
    await page
      .getByRole("textbox", { name: `New name for ${deviceName}` })
      .fill(renamedDeviceName);
    await deviceItem.getByRole("button", { name: "Save" }).click();
    await expect(page.locator("li", { hasText: renamedDeviceName })).toBeVisible();

    // Revoke it from the UI.
    await page.getByRole("button", { name: `Revoke ${renamedDeviceName}` }).click();
    await page
      .getByRole("alertdialog")
      .getByRole("button", { name: "Revoke" })
      .click();
    await expect(page.locator("li", { hasText: renamedDeviceName })).toHaveCount(0);

    // The backing state is gone too, not just the UI row.
    const list = await page.request.get("/api/devices", {
      failOnStatusCode: false,
    });
    expect(list.status()).toBe(200);
    const body = (await list.json()) as ApiResponse<DevicesData>;
    expect(body.data?.devices ?? []).toHaveLength(0);
  });
});
