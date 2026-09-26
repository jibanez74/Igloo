import { expect, test, type Page, type Route } from "@playwright/test";

import type {
  GeneralSettingsResponseType,
  GeneralSettingsType,
  UpdateGeneralSettingsRequest,
} from "../src/types";
import { trackBrowserIssues } from "./e2e-browser-issues";
import {
  readJSON,
} from "./e2e-api";
import { loginPageViaApi } from "./e2e-auth";

function requestFromSettings(
  settings: GeneralSettingsType,
): UpdateGeneralSettingsRequest {
  return {
    tmdb_key: settings.tmdb_key ?? "",
    immich_base_url: settings.immich_base_url ?? "",
    immich_api_key: settings.immich_api_key ?? "",
    jellyfin_base_url: settings.jellyfin_base_url ?? "",
    jellyfin_api_key: settings.jellyfin_api_key ?? "",
    spotify_client_id: settings.spotify_client_id ?? "",
    spotify_client_secret: settings.spotify_client_secret ?? "",
    enable_watcher: settings.enable_watcher,
    download_images: settings.download_images,
    static_dir: settings.static_dir,
    transcode_dir: settings.transcode_dir,
  };
}

async function fetchGeneralSettings(page: Page) {
  const response = await page.context().request.get("/api/settings/general", {
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(200);

  const body = await readJSON<GeneralSettingsResponseType>(response);
  expect(body.error, body.message).toBe(false);
  expect(body.data).toBeTruthy();
  return body.data!.settings;
}

async function restoreGeneralSettings(
  page: Page,
  settings: UpdateGeneralSettingsRequest,
) {
  const response = await page.context().request.put("/api/settings/general", {
    data: settings,
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(200);

  const body = await readJSON<unknown>(response);
  expect(body.error, body.message).toBe(false);
}

async function integrationFieldValues(page: Page) {
  return {
    jellyfin_base_url: await page
      .getByRole("textbox", { name: "Jellyfin base URL" })
      .inputValue(),
    jellyfin_api_key: await page
      .getByRole("textbox", { name: "Jellyfin API key" })
      .inputValue(),
    immich_base_url: await page
      .getByRole("textbox", { name: "Immich base URL" })
      .inputValue(),
    immich_api_key: await page
      .getByRole("textbox", { name: "Immich API key" })
      .inputValue(),
  };
}

async function expectNewIntegrationControls(page: Page) {
  await expect(
    page.getByRole("heading", { name: "Appearance" }),
  ).toHaveCount(0);
  await expect(page.getByText("Dark mode")).toHaveCount(0);
  await expect(
    page.getByRole("textbox", { name: "Jellyfin base URL" }),
  ).toBeVisible();
  await expect(
    page.getByRole("textbox", { name: "Jellyfin API key" }),
  ).toBeVisible();
  await expect(
    page.getByRole("textbox", { name: "Immich base URL" }),
  ).toBeVisible();
  await expect(
    page.getByRole("textbox", { name: "Immich API key" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Show Jellyfin API key" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Show Immich API key" }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "Reset" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Save Settings" })).toBeVisible();
}

async function expectScreenReaderSupport(page: Page) {
  await expect(
    page.getByRole("textbox", { name: "Jellyfin base URL" }),
  ).toHaveAccessibleDescription(/http:\/\/ or https:\/\//);
  await expect(
    page.getByRole("textbox", { name: "Immich base URL" }),
  ).toHaveAccessibleDescription(/http:\/\/ or https:\/\//);
  await expect(
    page.getByRole("textbox", { name: "Jellyfin API key" }),
  ).toHaveAccessibleDescription(/Leave blank to clear/);
  await expect(
    page.getByRole("textbox", { name: "Immich API key" }),
  ).toHaveAccessibleDescription(/Leave blank to clear/);

  await page.getByRole("textbox", { name: "Jellyfin base URL" }).fill(
    "ftp://not-valid.local",
  );
  await page.getByRole("button", { name: "Save Settings" }).click();
  await expect(
    page
      .getByRole("tabpanel", { name: "General" })
      .getByText("Jellyfin base URL must start with http:// or https://."),
  ).toBeVisible();
  await expect(
    page.getByRole("textbox", { name: "Jellyfin base URL" }),
  ).toHaveAttribute("aria-invalid", "true");
}

async function expectKeyboardFlow(page: Page) {
  await page.getByRole("textbox", { name: "TMDB API key" }).focus();

  for (const control of [
    page.getByRole("button", { name: "Show TMDB API key" }),
    page.getByRole("textbox", { name: "Jellyfin base URL" }),
    page.getByRole("textbox", { name: "Jellyfin API key" }),
    page.getByRole("button", { name: "Show Jellyfin API key" }),
    page.getByRole("textbox", { name: "Immich base URL" }),
    page.getByRole("textbox", { name: "Immich API key" }),
    page.getByRole("button", { name: "Show Immich API key" }),
  ]) {
    await page.keyboard.press("Tab");
    await expect(control).toBeFocused();
  }

  await page.getByRole("button", { name: "Show Jellyfin API key" }).focus();
  await page.keyboard.press("Space");
  await expect(
    page.getByRole("button", { name: "Hide Jellyfin API key" }),
  ).toBeVisible();
  await expect(
    page.getByRole("textbox", { name: "Jellyfin API key" }),
  ).toHaveAttribute("type", "text");

  await page.keyboard.press("Enter");
  await expect(
    page.getByRole("button", { name: "Show Jellyfin API key" }),
  ).toBeVisible();
  await expect(
    page.getByRole("textbox", { name: "Jellyfin API key" }),
  ).toHaveAttribute("type", "password");
}

async function fillIntegrationSettings(
  page: Page,
  settings: Pick<
    UpdateGeneralSettingsRequest,
    | "jellyfin_base_url"
    | "jellyfin_api_key"
    | "immich_base_url"
    | "immich_api_key"
  >,
) {
  await page
    .getByRole("textbox", { name: "Jellyfin base URL" })
    .fill(settings.jellyfin_base_url);
  await page
    .getByRole("textbox", { name: "Jellyfin API key" })
    .fill(settings.jellyfin_api_key);
  await page
    .getByRole("textbox", { name: "Immich base URL" })
    .fill(settings.immich_base_url);
  await page
    .getByRole("textbox", { name: "Immich API key" })
    .fill(settings.immich_api_key);
}

test.describe("General settings", () => {
  test("updates integration settings accessibly and optimistically", async ({
    page,
  }) => {
    const tracker = trackBrowserIssues(page);
    let settingsPutCount = 0;

    page.on("request", request => {
      if (
        request.method() === "PUT" &&
        new URL(request.url()).pathname === "/api/settings/general"
      ) {
        settingsPutCount += 1;
      }
    });

    await loginPageViaApi(page);
    const baselineSettings = await fetchGeneralSettings(page);
    const baselineRequest = requestFromSettings(baselineSettings);
    const stamp = Date.now();
    const nextSettings: UpdateGeneralSettingsRequest = {
      ...baselineRequest,
      jellyfin_base_url: `https://jellyfin-playwright-${stamp}.local:8096`,
      jellyfin_api_key: `playwright-jellyfin-key-${stamp}`,
      immich_base_url: `http://immich-playwright-${stamp}.local:2283`,
      immich_api_key: `playwright-immich-key-${stamp}`,
    };

    const delayed = {
      route: null as Route | null,
      body: null as UpdateGeneralSettingsRequest | null,
    };

    try {
      await page.goto("/settings");
      await expectNewIntegrationControls(page);
      await expectScreenReaderSupport(page);
      await expectKeyboardFlow(page);
      await fillIntegrationSettings(page, nextSettings);
      // The invalid URL was rejected before the mutation: nothing was sent.
      expect(settingsPutCount).toBe(0);

      await page.route("**/api/settings/general", async route => {
        const request = route.request();
        if (request.method() === "PUT" && delayed.route === null) {
          delayed.route = route;
          delayed.body = request.postDataJSON() as UpdateGeneralSettingsRequest;
          return;
        }

        await route.continue();
      });

      await page.getByRole("button", { name: "Save Settings" }).click();
      await expect
        .poll(() => delayed.route !== null, { timeout: 5_000 })
        .toBe(true);

      expect(delayed.body).toMatchObject({
        jellyfin_base_url: nextSettings.jellyfin_base_url,
        jellyfin_api_key: nextSettings.jellyfin_api_key,
        immich_base_url: nextSettings.immich_base_url,
        immich_api_key: nextSettings.immich_api_key,
      });

      await page.getByRole("tab", { name: "Account" }).click();
      await page.getByRole("tab", { name: "General" }).click();
      await expect(
        page.getByRole("textbox", { name: "Jellyfin base URL" }),
      ).toBeVisible();

      await expect
        .poll(() => integrationFieldValues(page))
        .toMatchObject({
          jellyfin_base_url: nextSettings.jellyfin_base_url,
          jellyfin_api_key: nextSettings.jellyfin_api_key,
          immich_base_url: nextSettings.immich_base_url,
          immich_api_key: nextSettings.immich_api_key,
        });

      const putResponsePromise = page.waitForResponse(
        response =>
          response.url().includes("/api/settings/general") &&
          response.request().method() === "PUT",
      );
      await delayed.route!.continue();
      delayed.route = null;

      const putResponse = await putResponsePromise;
      expect(putResponse.status()).toBe(200);
      const putBody = await readJSON<unknown>(putResponse);
      expect(putBody.error, putBody.message).toBe(false);

      await expect(page.getByText("Settings saved")).toBeVisible();

      const savedSettings = await fetchGeneralSettings(page);
      expect(savedSettings.jellyfin_base_url).toBe(
        nextSettings.jellyfin_base_url,
      );
      expect(savedSettings.jellyfin_api_key).toBe(
        nextSettings.jellyfin_api_key,
      );
      expect(savedSettings.immich_base_url).toBe(nextSettings.immich_base_url);
      expect(savedSettings.immich_api_key).toBe(nextSettings.immich_api_key);
    } finally {
      const routeToContinue = delayed.route;
      if (routeToContinue !== null) {
        await routeToContinue.continue().catch(() => undefined);
      }
      await page.unroute("**/api/settings/general").catch(() => undefined);
      await restoreGeneralSettings(page, baselineRequest);
    }

    tracker.assertClean();
  });
});
