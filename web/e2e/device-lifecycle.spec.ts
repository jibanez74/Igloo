import { expect, test } from "./e2e-test";
import type { APIRequestContext } from "@playwright/test";

import type { DevicesListResponseType } from "../src/types";
import type { components } from "../src/types/openapi.gen";
import { getApiData, readJSON } from "./e2e-api";
import { isApiResourceEcho } from "./e2e-browser-issues";
import { loginPageViaApi } from "./e2e-auth";
import { requireRealInstance } from "./e2e-env";

type InitiateData = components["schemas"]["QuickConnectInitiateData"];
type RedeemData = components["schemas"]["QuickConnectRedeemData"];

// Unique per run, so the flow neither trips over nor touches the devices a
// real instance already has.
const runId = Date.now().toString(36);
const deviceName = `E2E Living Room TV ${runId}`;
const renamedDeviceName = `E2E Bedroom TV ${runId}`;
const tokenDeviceName = `E2E Token TV ${runId}`;
const backedOutDeviceName = `E2E Kitchen TV ${runId}`;

async function initiate(
  request: APIRequestContext,
  name: string,
): Promise<InitiateData> {
  const response = await request.post("/api/quick-connect/initiate", {
    data: { device_name: name, platform: "android_tv", app_version: "e2e" },
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(201);

  const body = await readJSON<InitiateData>(response);
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

  const body = await readJSON<RedeemData>(response);
  expect(body.error).toBe(false);
  return body.data!;
}

/** Polls like the device does until the approved code yields its token. */
async function redeemApprovedToken(
  request: APIRequestContext,
  code: string,
  secret: string,
) {
  let token = "";
  await expect
    .poll(async () => {
      const result = await redeem(request, code, secret);
      token = result.token ?? "";
      return result.status;
    })
    .toBe("approved");
  return token;
}

async function fetchDevices(request: APIRequestContext) {
  const data = await getApiData<DevicesListResponseType>(request, "/api/devices");
  return data.devices;
}

/**
 * Revokes any device left behind under these names. Runs in `finally`, so it
 * asserts nothing: on the success path the device is already gone.
 */
async function revokeDevicesNamed(
  request: APIRequestContext,
  ...names: string[]
) {
  const response = await request.get("/api/devices", {
    failOnStatusCode: false,
  });
  if (!response.ok()) return;

  const body = await readJSON<DevicesListResponseType>(response);
  for (const device of body.data?.devices ?? []) {
    if (names.includes(device.name)) {
      await request.delete(`/api/devices/${device.id}`, {
        failOnStatusCode: false,
      });
    }
  }
}

test.describe("Device lifecycle", () => {
  test("pairs, renames, and revokes a device through the settings UI", async ({
    page,
    request,
  }) => {
    await loginPageViaApi(page);

    try {
      // The "device" (unauthenticated request context) starts pairing.
      const { code, secret } = await initiate(request, deviceName);

      const pending = await redeem(request, code, secret);
      expect(pending.status).toBe("pending");

      // The user enters the code and is shown which device is asking before
      // anything is approved.
      await page.goto("/settings/account");
      await page
        .getByRole("textbox", { name: "Quick Connect code" })
        .fill(code);
      await page.getByRole("button", { name: "Continue" }).click();
      await expect(
        page.getByRole("heading", { name: "Approve this device?" }),
      ).toBeVisible();
      await expect(page.getByText(deviceName)).toBeVisible();
      await page.getByRole("button", { name: "Approve device" }).click();

      // While the device finishes signing in, the card shows a waiting status.
      await expect(
        page
          .getByRole("status")
          .filter({ hasText: `Waiting for ${deviceName}` }),
      ).toBeVisible();

      await redeemApprovedToken(request, code, secret);

      // The card polls the devices list every 2 seconds while waiting, so the
      // new device shows up in the list shortly after it redeems its code.
      const devices = page
        .getByRole("list", { name: "Connected devices" })
        .getByRole("listitem");
      const deviceItem = devices.filter({ hasText: deviceName });
      await expect(deviceItem).toBeVisible();
      const device = (await fetchDevices(page.request)).find(
        (candidate) => candidate.name === deviceName,
      );
      expect(device).toBeDefined();

      // Rename it inline (scope Save to the row — the page has other Save buttons).
      await page.getByRole("button", { name: `Rename ${deviceName}` }).click();
      await page
        .getByRole("textbox", { name: `New name for ${deviceName}` })
        .fill(renamedDeviceName);
      await deviceItem.getByRole("button", { name: "Save" }).click();
      await expect(
        devices.filter({ hasText: renamedDeviceName }),
      ).toBeVisible();
      expect(
        (await fetchDevices(page.request)).find(
          (candidate) => candidate.id === device!.id,
        )?.name,
      ).toBe(renamedDeviceName);

      // Revoke it from the UI.
      await page
        .getByRole("button", { name: `Revoke ${renamedDeviceName}` })
        .click();
      await page
        .getByRole("alertdialog")
        .getByRole("button", { name: "Revoke" })
        .click();
      await expect(devices.filter({ hasText: renamedDeviceName })).toHaveCount(
        0,
      );

      // The backing state is gone too, not just the UI row.
      expect(
        (await fetchDevices(page.request)).some(
          (candidate) => candidate.id === device!.id,
        ),
      ).toBe(false);
    } finally {
      await revokeDevicesNamed(page.request, deviceName, renamedDeviceName);
    }
  });

  // The server's answers the card turns into messages: a code it never issued
  // (or one that expired, which it cannot tell apart) is a 404, and a
  // looked-up device stays unapproved until the reader approves it.
  test("refuses a code no device issued, and backing out approves nothing", async ({
    page,
    request,
    browserIssues,
  }) => {
    browserIssues.ignore({
      ignoreConsole: (_type, text, message) => isApiResourceEcho(text, message),
      ignoreResponse: response =>
        response.status() === 404 &&
        new URL(response.url()).pathname === "/api/quick-connect/lookup",
    });
    await loginPageViaApi(page);

    try {
      await page.goto("/settings/account");
      const codeInput = page.getByRole("textbox", { name: "Quick Connect code" });

      // Well formed, so only the server can refuse it.
      await codeInput.fill("ZZZZZZ");
      const refused = page.waitForResponse(
        response =>
          new URL(response.url()).pathname === "/api/quick-connect/lookup",
      );
      await page.getByRole("button", { name: "Continue" }).click();
      expect((await refused).status()).toBe(404);
      await expect(codeInput).toHaveAccessibleDescription(
        /That code is invalid or has expired\./,
      );

      const { code, secret } = await initiate(request, backedOutDeviceName);
      await codeInput.fill(code);
      await page.getByRole("button", { name: "Continue" }).click();
      await expect(page.getByText(backedOutDeviceName)).toBeVisible();
      await page.getByRole("button", { name: "Back" }).click();
      await expect(codeInput).toHaveValue(code);

      expect((await redeem(request, code, secret)).status).toBe("pending");
      expect(
        (await fetchDevices(page.request)).some(
          candidate => candidate.name === backedOutDeviceName,
        ),
      ).toBe(false);
    } finally {
      await revokeDevicesNamed(page.request, backedOutDeviceName);
    }
  });

  test("a paired device's token authenticates until it is revoked", async ({
    page,
    request,
  }) => {
    requireRealInstance("the mock server does not model device tokens");
    await loginPageViaApi(page);

    try {
      const { code, secret } = await initiate(request, tokenDeviceName);
      const approve = await page.request.post("/api/quick-connect/approve", {
        data: { code },
        failOnStatusCode: false,
      });
      expect(approve.status()).toBe(200);

      const token = await redeemApprovedToken(request, code, secret);
      expect(token).toMatch(/^igd_/);
      const authorization = { Authorization: `Bearer ${token}` };

      const me = await request.get("/api/auth/user", {
        headers: authorization,
        failOnStatusCode: false,
      });
      expect(me.status()).toBe(200);

      const device = (await fetchDevices(page.request)).find(
        (candidate) => candidate.name === tokenDeviceName,
      );
      expect(device).toBeDefined();
      const revoke = await page.request.delete(`/api/devices/${device!.id}`, {
        failOnStatusCode: false,
      });
      expect(revoke.status()).toBe(200);

      const revoked = await request.get("/api/auth/user", {
        headers: authorization,
        failOnStatusCode: false,
      });
      expect(revoked.status()).toBe(401);
    } finally {
      await revokeDevicesNamed(page.request, tokenDeviceName);
    }
  });
});
