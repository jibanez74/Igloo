import {
  expect,
  test,
  type Locator,
  type Page,
} from "@playwright/test";

import type {
  AdminUserType,
  DevicePlaybackPreferences,
  PlaybackSettingsResponseType,
  PlaybackSettingsType,
  TrailerPreferencesData,
  UpdatePlaybackSettingsRequest,
} from "../src/types";
import { PLAYBACK_PREFERENCES_STORAGE_PREFIX } from "../src/lib/playback-preferences";
import { trackBrowserIssues } from "./e2e-browser-issues";
import {
  readJSON,
} from "./e2e-api";
import { loginPageViaApi, logoutViaApi } from "./e2e-auth";
import { requireRealInstance } from "./e2e-env";
import { expectPageHasNoHorizontalScroll, VIEWPORTS } from "./e2e-layout";
import { createUser, deleteUser } from "./e2e-users";

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
  }, PLAYBACK_PREFERENCES_STORAGE_PREFIX);
}

const DOWNLOAD_SPEED_VALIDATION_MESSAGE =
  /Download speed must be between 0 and 10000 Mbps\./;

async function fetchPlaybackSettings(page: Page) {
  const response = await page.context().request.get("/api/settings/playback", {
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(200);

  const body = await readJSON<PlaybackSettingsResponseType>(response);
  expect(body.error, body.message).toBe(false);
  expect(body.data?.settings).toBeTruthy();
  return body.data!.settings;
}

async function restorePlaybackSettings(
  page: Page,
  settings: PlaybackSettingsType,
) {
  const response = await page.context().request.put("/api/settings/playback", {
    data: {
      server_upload_mbps: settings.server_upload_mbps,
      hardware_acceleration_device: settings.hardware_acceleration_device,
    } satisfies UpdatePlaybackSettingsRequest,
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(200);

  const body = await readJSON<unknown>(response);
  expect(body.error, body.message).toBe(false);
}

const TRAILER_PREFERENCES_PATH = "/api/user/preferences/trailers";

async function fetchTrailerPreferences(page: Page) {
  const response = await page.context().request.get(TRAILER_PREFERENCES_PATH, {
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(200);
  const body = await readJSON<TrailerPreferencesData>(response);
  expect(body.error, body.message).toBe(false);
  return body.data!;
}

type RegularUser = AdminUserType & { password: string };

/** A disposable non-admin account, created through the page's admin session. */
async function createRegularUser(page: Page): Promise<RegularUser> {
  const stamp = Date.now();
  const password = `PlaybackPass${stamp}!`;
  const user = await createUser(page.context().request, {
    name: `Playback Settings User ${stamp}`,
    email: `playback-settings-${stamp}@example.com`,
    password,
  });
  return { ...user, password };
}

async function signBackInAsAdmin(page: Page, user: RegularUser | null) {
  await logoutViaApi(page.context().request);
  await loginPageViaApi(page);
  if (user) {
    await deleteUser(page.context().request, user.id);
  }
}

async function expectTabMovesFocus(page: Page, next: Locator) {
  await page.keyboard.press("Tab");
  await expect(next).toBeFocused();
}

test.describe("Playback settings", () => {
  test("saves server playback settings and applies device preferences instantly", async ({
    page,
  }) => {
    const tracker = trackBrowserIssues(page);

    await loginPageViaApi(page);
    const baselineSettings = await fetchPlaybackSettings(page);

    try {
      await page.goto("/settings/playback");
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
      await expect(downloadInput).toHaveAccessibleDescription(/Leave blank/);
      await expect(serverInput).toHaveAccessibleDescription(/Leave blank/);

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
      // The account-scoped trailer card sits between the device cards and
      // the admin Server card.
      await expectTabMovesFocus(
        page,
        page.getByRole("switch", { name: "Play trailers before movies" }),
      );
      await expectTabMovesFocus(
        page,
        page.getByRole("combobox", { name: "Number of trailers" }),
      );
      await expectTabMovesFocus(
        page,
        page.getByRole("combobox", { name: "Trailer source" }),
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
      await expect(page.getByText("Recommended: 2160p · 16 Mbps")).toBeVisible();

      await page
        .getByRole("combobox", { name: "Hardware acceleration" })
        .click();
      await page.getByRole("option", { name: "NVIDIA NVENC" }).click();

      await page.setViewportSize({ width: 360, height: 800 });
      await expect(page.getByRole("button", { name: "Reset" })).toBeVisible();
      await expect(
        page.getByRole("button", { name: "Save Settings" }),
      ).toBeVisible();
      await expectPageHasNoHorizontalScroll(page);
      await page.setViewportSize(VIEWPORTS.desktop);

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
      // Only server-owned fields travel over the wire now.
      const putBody =
        putResponse.request().postDataJSON() as UpdatePlaybackSettingsRequest;
      expect(putBody).toMatchObject({
        server_upload_mbps: 5,
        hardware_acceleration_device: "nvidia",
      });
      expect(putBody).not.toHaveProperty("download_mbps");

      await expect(page.getByText("Playback settings saved")).toBeVisible();
      const savedSettings = await fetchPlaybackSettings(page);
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

      await page.reload();
      await expect(downloadInput).toHaveValue("100");
      await expect(
        page.getByRole("combobox", { name: "Subtitle language" }),
      ).toHaveText("Always off");
    } finally {
      await restorePlaybackSettings(page, baselineSettings);
    }

    tracker.assertClean();
  });

  test("saves trailer pre-roll preferences to the account as they change", async ({
    page,
  }) => {
    const tracker = trackBrowserIssues(page);
    let regularUser: RegularUser | null = null;

    await loginPageViaApi(page);

    try {
      // A fresh account, so the defaults hold however a real instance's
      // admin has set its own trailers, and nothing is left on it.
      regularUser = await createRegularUser(page);
      await logoutViaApi(page.context().request);
      await loginPageViaApi(page, regularUser);
      const baseline = await fetchTrailerPreferences(page);
      expect(baseline.enabled).toBe(false);

      await page.goto("/settings/playback");
      await expect(
        page.getByRole("heading", { name: "Trailers before movies" }),
      ).toBeVisible();
      await expect(page.getByText(/Saved to your account, so it follows/)).toBeVisible();

      const toggle = page.getByRole("switch", { name: "Play trailers before movies" });
      await expect(toggle).toHaveAttribute("aria-checked", "false");
      await toggle.click();
      await expect(toggle).toHaveAttribute("aria-checked", "true");
      await expect(
        page.getByText("Trailers before movies turned on. Saved to your account."),
      ).toBeVisible();

      await page.getByRole("combobox", { name: "Number of trailers" }).click();
      await page.getByRole("option", { name: "3 trailers" }).click();
      await expect(
        page.getByText("Trailer count set to 3. Saved to your account."),
      ).toBeVisible();

      await page.getByRole("combobox", { name: "Trailer source" }).click();
      await page.getByRole("option", { name: "In theaters" }).click();
      await expect(
        page.getByText("Trailer source set to In theaters. Saved to your account."),
      ).toBeVisible();

      await expect.poll(() => fetchTrailerPreferences(page)).toEqual({
        enabled: true,
        count: 3,
        source: "theaters",
      });

      // The account holds the values, so a reload shows them again.
      await page.reload();
      await expect(
        page.getByRole("switch", { name: "Play trailers before movies" }),
      ).toHaveAttribute("aria-checked", "true");
      await expect(
        page.getByRole("combobox", { name: "Number of trailers" }),
      ).toHaveText("3 trailers");
      await expect(
        page.getByRole("combobox", { name: "Trailer source" }),
      ).toHaveText("In theaters");
    } finally {
      await signBackInAsAdmin(page, regularUser);
    }

    tracker.assertClean();
  });

  test("rejects an out-of-range download speed without storing it", async ({
    page,
  }) => {
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

    await loginPageViaApi(page);

    await page.goto("/settings/playback");
    const downloadInput = page.getByRole("spinbutton", {
      name: "Download speed (Mbps)",
    });
    const validationStatus = page.getByText(DOWNLOAD_SPEED_VALIDATION_MESSAGE);

    await downloadInput.fill("0");
    await expect(validationStatus).toBeVisible();
    await expect(downloadInput).toHaveAttribute("aria-invalid", "true");
    await expect(downloadInput).toHaveAccessibleDescription(
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

    // The upper bound is enforced too, and the last valid value stays stored.
    await downloadInput.fill("10001");
    await expect(validationStatus).toBeVisible();
    await expect(downloadInput).toHaveAttribute("aria-invalid", "true");
    expect((await readDevicePreferences(page))?.downloadMbps).toBe(5);
    expect(playbackPutCount).toBe(0);

    tracker.assertClean();
  });

  // Regression for the cross-tab clobbering in docs/ls-review.md: each tab
  // caches its own snapshot, so a merge against a stale one used to write the
  // other tab's field away.
  test("keeps preferences set in another tab", async ({ page, context }) => {
    const tracker = trackBrowserIssues(page);

    await loginPageViaApi(page);

    const second = await context.newPage();
    const secondTracker = trackBrowserIssues(second);

    try {
      await page.goto("/settings/playback");
      await expect(
        page.getByRole("heading", { name: "Streaming & bandwidth" }),
      ).toBeVisible();

      await second.goto("/settings/playback");
      await expect(
        second.getByRole("heading", { name: "Stream defaults" }),
      ).toBeVisible();

      // Tab one sets the audio language.
      await page.getByRole("combobox", { name: "Audio language" }).click();
      await page.getByRole("option", { name: "English", exact: true }).click();
      await expect
        .poll(async () => (await readDevicePreferences(page))?.preferredAudioLanguage)
        .toBe("en");

      // Tab two, holding a snapshot from before that write, sets a different field.
      await second.getByRole("spinbutton", { name: "Download speed (Mbps)" }).fill("30");
      await expect
        .poll(async () => (await readDevicePreferences(second))?.downloadMbps)
        .toBe(30);

      // Both survive, and tab two shows the language tab one chose.
      const stored = await readDevicePreferences(second);
      expect(stored?.preferredAudioLanguage).toBe("en");
      expect(stored?.downloadMbps).toBe(30);
      await expect(
        second.getByRole("combobox", { name: "Audio language" }),
      ).toHaveText("English");
    } finally {
      await second.close();
    }

    secondTracker.assertClean();
    tracker.assertClean();
  });

  test("gives regular users device-only settings with no save bar", async ({
    page,
  }) => {
    const tracker = trackBrowserIssues(page);
    let regularUser: RegularUser | null = null;
    let playbackPutSent = false;

    page.on("request", request => {
      const url = new URL(request.url());
      if (
        url.pathname === "/api/settings/playback" &&
        request.method() === "PUT"
      ) {
        playbackPutSent = true;
      }
    });

    await loginPageViaApi(page);

    try {
      regularUser = await createRegularUser(page);
      await logoutViaApi(page.context().request);
      await loginPageViaApi(page, regularUser);

      await page.goto("/settings/playback");
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

      await page
        .getByRole("spinbutton", { name: "Download speed (Mbps)" })
        .fill("30");

      await expect
        .poll(async () => (await readDevicePreferences(page))?.downloadMbps)
        .toBe(30);
      expect(playbackPutSent).toBe(false);
    } finally {
      await signBackInAsAdmin(page, regularUser);
    }

    tracker.assertClean();
  });

  test("rejects a regular user's direct write to the server settings", async ({
    page,
  }) => {
    requireRealInstance("the mock server's admin check is not the one under test");
    let regularUser: RegularUser | null = null;

    await loginPageViaApi(page);

    try {
      regularUser = await createRegularUser(page);
      await logoutViaApi(page.context().request);
      await loginPageViaApi(page, regularUser);

      // RequireAdmin rejects a direct PUT regardless of which field it carries.
      for (const data of [
        { server_upload_mbps: 9 },
        { hardware_acceleration_device: "nvidia" as const },
      ] satisfies UpdatePlaybackSettingsRequest[]) {
        const forbidden = await page.context().request.put(
          "/api/settings/playback",
          { data, failOnStatusCode: false },
        );
        expect(forbidden.status()).toBe(403);
      }
    } finally {
      await signBackInAsAdmin(page, regularUser);
    }
  });
});
