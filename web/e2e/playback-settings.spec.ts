import {
  expect,
  test,
  type APIResponse,
  type Locator,
  type Page,
  type Response,
} from "@playwright/test";

import { apiURL, readE2EEnv, type E2EEnv } from "./e2e-env";
import { isIgnorableFailedRequest } from "./e2e-browser-issues";

type ApiResponse<T> = {
  error: boolean;
  message?: string;
  data?: T;
};

type PlaybackProfile = {
  id: string;
  label: string;
  height: number;
  video_mbps: number;
};

type HardwareAccelerationDevice = "cpu" | "apple" | "nvidia" | "intel";

type PlaybackSettings = {
  profiles: PlaybackProfile[];
  server_upload_mbps: number | null;
  hardware_acceleration_device: HardwareAccelerationDevice;
};

type PlaybackSettingsData = {
  settings: PlaybackSettings;
};

type UpdatePlaybackSettingsRequest = {
  server_upload_mbps?: number | null;
  hardware_acceleration_device?: HardwareAccelerationDevice;
};

/** Mirrors DevicePlaybackPreferences in src/lib/playback-preferences.ts. */
type DevicePlaybackPreferences = {
  preferredProfile: string | null;
  downloadMbps: number | null;
  preferredAudioLanguage: string | null;
  preferredSubtitleLanguage: string | null;
};

const DEVICE_PREFS_STORAGE_PREFIX = "igloo-playback-prefs:";

async function readDevicePreferences(page: Page) {
  return page.evaluate(prefix => {
    const key = Object.keys(localStorage).find(name =>
      name.startsWith(prefix),
    );
    return key
      ? (JSON.parse(localStorage.getItem(key) ?? "null") as
          | DevicePlaybackPreferences
          | null)
      : null;
  }, DEVICE_PREFS_STORAGE_PREFIX);
}

async function clearDevicePreferences(page: Page) {
  await page.evaluate(prefix => {
    for (const name of Object.keys(localStorage)) {
      if (name.startsWith(prefix)) localStorage.removeItem(name);
    }
  }, DEVICE_PREFS_STORAGE_PREFIX);
}

type AdminUser = {
  id: number;
  name: string;
  email: string;
  is_admin: boolean;
  avatar: string | null;
  created_at: string;
  updated_at: string;
};

type AdminCreateUserData = {
  user: AdminUser;
};

const DOWNLOAD_SPEED_VALIDATION_MESSAGE =
  "Download speed must be between 0 and 10000 Mbps.";

async function readJSON<T>(response: APIResponse | Response) {
  return (await response.json()) as ApiResponse<T>;
}

function expectDefined<T>(value: T, message: string): NonNullable<T> {
  expect(value, message).not.toBeNull();
  return value as NonNullable<T>;
}

async function login(
  page: Page,
  env: E2EEnv,
  email = env.email,
  password = env.password,
) {
  const response = await page.context().request.post(apiURL(env, "/api/auth/login"), {
    data: { email, password },
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(200);

  const body = await readJSON<unknown>(response);
  expect(body.error, body.message).toBe(false);
}

async function logout(page: Page, env: E2EEnv) {
  await page.context().request.delete(apiURL(env, "/api/auth/logout"), {
    failOnStatusCode: false,
  });
}

async function fetchPlaybackSettings(page: Page, env: E2EEnv) {
  const response = await page.context().request.get(
    apiURL(env, "/api/settings/playback"),
    { failOnStatusCode: false },
  );
  expect(response.status()).toBe(200);

  const body = await readJSON<PlaybackSettingsData>(response);
  expect(body.error, body.message).toBe(false);
  expect(body.data?.settings).toBeTruthy();
  return body.data!.settings;
}

async function restorePlaybackSettings(
  page: Page,
  env: E2EEnv,
  settings: PlaybackSettings,
) {
  const response = await page.context().request.put(
    apiURL(env, "/api/settings/playback"),
    {
      data: {
        server_upload_mbps: settings.server_upload_mbps,
        hardware_acceleration_device: settings.hardware_acceleration_device,
      } satisfies UpdatePlaybackSettingsRequest,
      failOnStatusCode: false,
    },
  );
  expect(response.status()).toBe(200);

  const body = await readJSON<unknown>(response);
  expect(body.error, body.message).toBe(false);
}

async function createRegularUser(
  page: Page,
  env: E2EEnv,
  stamp: number,
) {
  const response = await page.context().request.post(
    apiURL(env, "/api/admin/users"),
    {
      data: {
        name: `Playback Settings User ${stamp}`,
        email: `playback-settings-${stamp}@example.com`,
        password: `PlaybackPass${stamp}!`,
        is_admin: false,
      },
      failOnStatusCode: false,
    },
  );
  expect(response.status()).toBe(201);

  const body = await readJSON<AdminCreateUserData>(response);
  expect(body.error, body.message).toBe(false);
  return {
    user: body.data!.user,
    password: `PlaybackPass${stamp}!`,
  };
}

async function deleteUser(page: Page, env: E2EEnv, userId: number) {
  const response = await page.context().request.delete(
    apiURL(env, `/api/admin/users/${userId}`),
    { failOnStatusCode: false },
  );
  expect(response.status()).toBe(200);
}

function isAppApiResponse(response: Response) {
  return new URL(response.url()).pathname.startsWith("/api/");
}

function trackBrowserIssues(page: Page) {
  const consoleIssues: string[] = [];
  const pageErrors: string[] = [];
  const failedRequests: string[] = [];
  const responseErrors: string[] = [];

  page.on("console", message => {
    if (message.type() === "error" || message.type() === "warning") {
      consoleIssues.push(`${message.type()}: ${message.text()}`);
    }
  });
  page.on("pageerror", error => pageErrors.push(error.message));
  page.on("requestfailed", request => {
    if (isIgnorableFailedRequest(request)) {
      return;
    }

    failedRequests.push(
      `${request.method()} ${request.url()} ${request.failure()?.errorText ?? ""}`,
    );
  });
  page.on("response", response => {
    if (isAppApiResponse(response) && response.status() >= 500) {
      responseErrors.push(
        `${response.status()} ${response.request().method()} ${response.url()}`,
      );
    }
  });

  return {
    assertClean() {
      expect(consoleIssues).toEqual([]);
      expect(pageErrors).toEqual([]);
      expect(failedRequests).toEqual([]);
      expect(responseErrors).toEqual([]);
    },
  };
}

async function expectDescriptionIncludes(
  page: Page,
  locator: Locator,
  expectedText: string,
) {
  const describedBy = await locator.getAttribute("aria-describedby");
  expect(describedBy).toBeTruthy();

  const descriptionText = await page.evaluate(ids => {
    return ids
      .split(/\s+/)
      .map(id => document.getElementById(id)?.textContent?.trim() ?? "")
      .filter(Boolean)
      .join(" ");
  }, describedBy ?? "");

  expect(descriptionText).toContain(expectedText);
}

async function expectNoHorizontalOverflow(page: Page) {
  await expect
    .poll(() =>
      page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth + 1,
      ),
    )
    .toBe(true);
}

async function expectTabMovesFocus(page: Page, next: Locator) {
  await page.keyboard.press("Tab");
  await expect(next).toBeFocused();
}

test.describe.configure({ mode: "serial" });

test.describe("Playback settings", () => {
  test("saves server playback settings and applies device preferences instantly", async ({
    page,
  }) => {
    const env = readE2EEnv();
    const tracker = trackBrowserIssues(page);
    const capturedRequest = {
      body: null as UpdatePlaybackSettingsRequest | null,
    };

    page.on("request", request => {
      const url = new URL(request.url());
      if (
        url.pathname === "/api/settings/playback" &&
        request.method() === "PUT"
      ) {
        capturedRequest.body =
          request.postDataJSON() as UpdatePlaybackSettingsRequest;
      }
    });

    await login(page, env);
    const baselineSettings = await fetchPlaybackSettings(page, env);

    try {
      await page.goto(apiURL(env, "/settings/playback"), {
        waitUntil: "networkidle",
      });
      await expect(
        page.getByRole("heading", { name: "Streaming & bandwidth" }),
      ).toBeVisible();
      await expect(
        page.getByRole("heading", { name: "Stream defaults" }),
      ).toBeVisible();
      await expect(
        page.getByRole("heading", { name: "Transcoding" }),
      ).toBeVisible();
      await expect(page.getByRole("tab", { name: "Playback" })).toBeVisible();

      const downloadInput = page.getByRole("spinbutton", {
        name: "Download speed (Mbps)",
      });
      const serverInput = page.getByRole("spinbutton", {
        name: "Server upload bandwidth (Mbps)",
      });

      await expect(downloadInput).toBeVisible();
      await expect(serverInput).toBeVisible();
      await expectDescriptionIncludes(page, downloadInput, "Leave blank");
      await expectDescriptionIncludes(page, serverInput, "Leave blank");

      await downloadInput.focus();
      await expect(downloadInput).toBeFocused();
      await expectTabMovesFocus(
        page,
        page.getByRole("combobox", { name: "Profile" }),
      );
      await expectTabMovesFocus(
        page,
        page.getByRole("combobox", { name: "Audio language" }),
      );
      await expectTabMovesFocus(
        page,
        page.getByRole("combobox", { name: "Subtitle language" }),
      );
      await expectTabMovesFocus(page, serverInput);
      await expectTabMovesFocus(
        page,
        page.getByRole("combobox", { name: "Hardware acceleration" }),
      );
      await expectTabMovesFocus(page, page.getByRole("button", { name: "Reset" }));
      await expectTabMovesFocus(
        page,
        page.getByRole("button", { name: "Save Settings" }),
      );

      await downloadInput.fill("100");
      await serverInput.fill("5");
      // The recommendation tracks the download speed immediately; the server
      // cap only counts once it is actually saved, below.
      await expect(page.getByText("Recommended: 4K · 16 Mbps")).toBeVisible();

      await page
        .getByRole("combobox", { name: "Hardware acceleration" })
        .click();
      await page.getByRole("option", { name: "NVIDIA NVENC" }).click();

      await page.setViewportSize({ width: 360, height: 800 });
      await expect(page.getByRole("button", { name: "Reset" })).toBeVisible();
      await expect(
        page.getByRole("button", { name: "Save Settings" }),
      ).toBeVisible();
      await expectNoHorizontalOverflow(page);
      await page.setViewportSize({ width: 1440, height: 900 });

      const putResponsePromise = page.waitForResponse(response => {
        const url = new URL(response.url());
        return (
          url.pathname === "/api/settings/playback" &&
          response.request().method() === "PUT"
        );
      });
      await page.getByRole("button", { name: "Save Settings" }).click();

      const putResponse = await putResponsePromise;
      expect(putResponse.status()).toBe(200);
      const capturedPutBody = expectDefined(
        capturedRequest.body,
        "Expected playback settings request body to be captured.",
      );
      // Only server-owned fields travel over the wire now.
      expect(capturedPutBody.server_upload_mbps).toBe(5);
      expect(capturedPutBody.hardware_acceleration_device).toBe("nvidia");
      expect(
        Object.prototype.hasOwnProperty.call(capturedPutBody, "download_mbps"),
      ).toBe(false);

      await expect(page.getByText("Playback settings saved")).toBeVisible();
      const savedSettings = await fetchPlaybackSettings(page, env);
      expect(savedSettings.server_upload_mbps).toBe(5);
      expect(savedSettings.hardware_acceleration_device).toBe("nvidia");

      // Saved, the server cap now clamps this device's recommendation.
      await expect(page.getByText("Recommended: 1080p · 4 Mbps")).toBeVisible();

      // The device half never touched the API; it is in local storage and
      // survives a reload.
      await page
        .getByRole("combobox", { name: "Subtitle language" })
        .click();
      await page.getByRole("option", { name: "Always off" }).click();

      await expect
        .poll(async () => (await readDevicePreferences(page))?.downloadMbps)
        .toBe(100);
      await expect
        .poll(
          async () =>
            (await readDevicePreferences(page))?.preferredSubtitleLanguage,
        )
        .toBe("off");

      await page.reload({ waitUntil: "networkidle" });
      await expect(downloadInput).toHaveValue("100");
      await expect(
        page.getByRole("combobox", { name: "Subtitle language" }),
      ).toHaveText("Always off");
      await clearDevicePreferences(page);
    } finally {
      await restorePlaybackSettings(page, env, baselineSettings);
    }

    tracker.assertClean();
  });

  test("rejects an out-of-range download speed without storing it", async ({
    page,
  }) => {
    const env = readE2EEnv();
    const tracker = trackBrowserIssues(page);
    let playbackPutCount = 0;

    page.on("request", request => {
      const url = new URL(request.url());
      if (
        url.pathname === "/api/settings/playback" &&
        request.method() === "PUT"
      ) {
        playbackPutCount += 1;
      }
    });

    await login(page, env);

    await page.goto(apiURL(env, "/settings/playback"), {
      waitUntil: "networkidle",
    });
    const downloadInput = page.getByRole("spinbutton", {
      name: "Download speed (Mbps)",
    });

    await clearDevicePreferences(page);
    await downloadInput.fill("0");

    const validationStatus = page
      .locator("p[aria-live='polite']")
      .filter({ hasText: DOWNLOAD_SPEED_VALIDATION_MESSAGE });
    await expect(validationStatus).toBeVisible();
    await expect(validationStatus).toHaveAttribute("aria-live", "polite");
    await expect(downloadInput).toHaveAttribute("aria-invalid", "true");
    await expectDescriptionIncludes(
      page,
      downloadInput,
      DOWNLOAD_SPEED_VALIDATION_MESSAGE,
    );
    // Device preferences never hit the API, and an invalid value is not stored.
    expect(playbackPutCount).toBe(0);
    expect((await readDevicePreferences(page))?.downloadMbps ?? null).toBeNull();

    await downloadInput.fill("5");
    await expect(validationStatus).toHaveCount(0);
    await expect(downloadInput).not.toHaveAttribute("aria-invalid", "true");
    await expect
      .poll(async () => (await readDevicePreferences(page))?.downloadMbps)
      .toBe(5);

    await clearDevicePreferences(page);
    tracker.assertClean();
  });

  test("gives regular users device-only settings with no save bar", async ({
    page,
  }) => {
    const env = readE2EEnv();
    const stamp = Date.now();
    const tracker = trackBrowserIssues(page);
    let regularUser: AdminUser | null = null;
    let regularPassword = "";
    const capturedRequest = {
      body: null as UpdatePlaybackSettingsRequest | null,
    };

    page.on("request", request => {
      const url = new URL(request.url());
      if (
        url.pathname === "/api/settings/playback" &&
        request.method() === "PUT"
      ) {
        capturedRequest.body =
          request.postDataJSON() as UpdatePlaybackSettingsRequest;
      }
    });

    await login(page, env);
    const baselineSettings = await fetchPlaybackSettings(page, env);

    try {
      const created = await createRegularUser(page, env, stamp);
      regularUser = created.user;
      regularPassword = created.password;

      await logout(page, env);
      await login(page, env, regularUser.email, regularPassword);

      await page.goto(apiURL(env, "/settings/playback"), {
        waitUntil: "networkidle",
      });
      await expect(page.getByRole("tab", { name: "Playback" })).toBeVisible();
      await expect(
        page.getByRole("spinbutton", {
          name: "Server upload bandwidth (Mbps)",
        }),
      ).toHaveCount(0);
      await expect(
        page.getByRole("heading", { name: "Transcoding" }),
      ).toHaveCount(0);
      await expect(
        page.getByRole("combobox", { name: "Hardware acceleration" }),
      ).toHaveCount(0);
      await expect(
        page.getByText(
          "Set by the server administrator. Affects your recommendation",
        ),
      ).toBeVisible();

      // Nothing on this tab writes to the server for a regular user.
      await expect(
        page.getByRole("button", { name: "Save Settings" }),
      ).toHaveCount(0);
      await expect(page.getByRole("button", { name: "Reset" })).toHaveCount(0);

      await clearDevicePreferences(page);
      await page
        .getByRole("spinbutton", { name: "Download speed (Mbps)" })
        .fill("30");

      await expect
        .poll(async () => (await readDevicePreferences(page))?.downloadMbps)
        .toBe(30);
      expect(capturedRequest.body).toBeNull();

      // RequireAdmin rejects a direct PUT regardless of which field it carries.
      for (const data of [
        { server_upload_mbps: 9 },
        { hardware_acceleration_device: "nvidia" as const },
      ] satisfies UpdatePlaybackSettingsRequest[]) {
        const forbidden = await page.context().request.put(
          apiURL(env, "/api/settings/playback"),
          { data, failOnStatusCode: false },
        );
        expect(forbidden.status()).toBe(403);
      }

      await clearDevicePreferences(page);
    } finally {
      await logout(page, env);
      await login(page, env);
      await restorePlaybackSettings(page, env, baselineSettings);
      if (regularUser) {
        await deleteUser(page, env, regularUser.id);
      }
    }

    tracker.assertClean();
  });
});
