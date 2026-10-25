import { expect, test, type Page, type Route } from "@playwright/test";

import type {
  GeneralSettingsResponseType,
  GeneralSettingsType,
  UpdateGeneralSettingsRequest,
} from "../src/types";
import { trackBrowserIssues } from "./e2e-browser-issues";
import { getApiData, readJSON } from "./e2e-api";
import { loginPageViaApi } from "./e2e-auth";

function requestFromSettings(
  settings: GeneralSettingsType,
): UpdateGeneralSettingsRequest {
  return {
    tmdb_key: settings.tmdb_key ?? "",
    spotify_client_id: settings.spotify_client_id ?? "",
    spotify_client_secret: settings.spotify_client_secret ?? "",
    enable_apple_lookups: settings.enable_apple_lookups,
    enable_watcher: settings.enable_watcher,
    download_images: settings.download_images,
    static_dir: settings.static_dir,
    transcode_dir: settings.transcode_dir,
  };
}

async function fetchGeneralSettings(page: Page) {
  const data = await getApiData<GeneralSettingsResponseType>(
    page.context().request,
    "/api/settings/general",
  );
  return data.settings;
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

async function serviceFieldValues(page: Page) {
  return {
    spotify_client_id: await page
      .getByRole("textbox", { name: "Spotify client ID" })
      .inputValue(),
    spotify_client_secret: await page
      .getByRole("textbox", { name: "Spotify client secret" })
      .inputValue(),
  };
}

async function expectServiceControls(page: Page) {
  await expect(
    page.getByRole("heading", { name: "Appearance" }),
  ).toHaveCount(0);
  await expect(page.getByText("Dark mode")).toHaveCount(0);
  for (const label of [
    "TMDB API key",
    "Spotify client ID",
    "Spotify client secret",
  ]) {
    await expect(page.getByRole("textbox", { name: label })).toBeVisible();
    await expect(
      page.getByRole("button", { name: `Show ${label}` }),
    ).toBeVisible();
  }
  // Both wait, disabled, until something changes.
  await expect(page.getByRole("button", { name: "Reset" })).toBeDisabled();
  await expect(page.getByRole("button", { name: "Save Settings" })).toBeDisabled();
}

async function expectScreenReaderSupport(page: Page, staticDir: string) {
  await expect(
    page.getByRole("textbox", { name: "TMDB API key" }),
  ).toHaveAccessibleDescription(/Leave blank to clear/);
  await expect(
    page.getByRole("textbox", { name: "Spotify client secret" }),
  ).toHaveAccessibleDescription(/Leave blank to clear/);
  await expect(
    page.getByRole("switch", { name: "Apple Music lookups" }),
  ).toHaveAccessibleDescription(/No account needed/);

  const staticDirectory = page.getByRole("textbox", {
    name: "Static directory",
  });
  await staticDirectory.fill("");
  await page.getByRole("button", { name: "Save Settings" }).click();
  await expect(
    page
      .getByRole("tabpanel", { name: "General" })
      .getByText("Static directory is required."),
  ).toBeVisible();
  await expect(staticDirectory).toHaveAttribute("aria-invalid", "true");

  await staticDirectory.fill(staticDir);
  await expect(staticDirectory).not.toHaveAttribute("aria-invalid", "true");
}

async function expectKeyboardFlow(page: Page) {
  await page.getByRole("textbox", { name: "TMDB API key" }).focus();

  for (const control of [
    page.getByRole("button", { name: "Show TMDB API key" }),
    page.getByRole("switch", { name: "Apple Music lookups" }),
    page.getByRole("textbox", { name: "Spotify client ID" }),
    page.getByRole("button", { name: "Show Spotify client ID" }),
    page.getByRole("textbox", { name: "Spotify client secret" }),
  ]) {
    await page.keyboard.press("Tab");
    await expect(control).toBeFocused();
  }

  await page.getByRole("button", { name: "Show Spotify client ID" }).focus();
  await page.keyboard.press("Space");
  await expect(
    page.getByRole("button", { name: "Hide Spotify client ID" }),
  ).toBeVisible();
  await expect(
    page.getByRole("textbox", { name: "Spotify client ID" }),
  ).toHaveAttribute("type", "text");

  await page.keyboard.press("Enter");
  await expect(
    page.getByRole("button", { name: "Show Spotify client ID" }),
  ).toBeVisible();
  await expect(
    page.getByRole("textbox", { name: "Spotify client ID" }),
  ).toHaveAttribute("type", "password");
}

async function fillServiceSettings(
  page: Page,
  settings: Pick<
    UpdateGeneralSettingsRequest,
    "spotify_client_id" | "spotify_client_secret"
  >,
) {
  await page
    .getByRole("textbox", { name: "Spotify client ID" })
    .fill(settings.spotify_client_id);
  await page
    .getByRole("textbox", { name: "Spotify client secret" })
    .fill(settings.spotify_client_secret);
}

test.describe("General settings", () => {
  test("updates service settings accessibly and optimistically", async ({
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
      spotify_client_id: `playwright-spotify-id-${stamp}`,
      spotify_client_secret: `playwright-spotify-secret-${stamp}`,
      enable_apple_lookups: !baselineRequest.enable_apple_lookups,
    };

    const delayed = {
      route: null as Route | null,
      body: null as UpdateGeneralSettingsRequest | null,
    };

    try {
      await page.goto("/settings");
      await expectServiceControls(page);
      await expectScreenReaderSupport(page, baselineRequest.static_dir);
      await expectKeyboardFlow(page);
      await fillServiceSettings(page, nextSettings);
      const appleLookups = page.getByRole("switch", { name: "Apple Music lookups" });
      await appleLookups.click();
      await expect(appleLookups).toHaveAttribute(
        "aria-checked",
        String(nextSettings.enable_apple_lookups),
      );
      // The blank static directory was rejected before the mutation: nothing was sent.
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
        spotify_client_id: nextSettings.spotify_client_id,
        spotify_client_secret: nextSettings.spotify_client_secret,
        enable_apple_lookups: nextSettings.enable_apple_lookups,
      });

      await page.getByRole("tab", { name: "Account" }).click();
      await page.getByRole("tab", { name: "General" }).click();
      await expect(
        page.getByRole("textbox", { name: "Spotify client ID" }),
      ).toBeVisible();

      await expect
        .poll(() => serviceFieldValues(page))
        .toMatchObject({
          spotify_client_id: nextSettings.spotify_client_id,
          spotify_client_secret: nextSettings.spotify_client_secret,
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
      expect(savedSettings.spotify_client_id).toBe(
        nextSettings.spotify_client_id,
      );
      expect(savedSettings.spotify_client_secret).toBe(
        nextSettings.spotify_client_secret,
      );
      expect(savedSettings.enable_apple_lookups).toBe(
        nextSettings.enable_apple_lookups,
      );
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
