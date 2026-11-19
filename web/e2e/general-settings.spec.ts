import { expect, test } from "./e2e-test";
import type { Page, Route } from "@playwright/test";

import type {
  GeneralSettingsResponseType,
  GeneralSettingsType,
  UpdateGeneralSettingsRequest,
} from "../src/types";
import { getApiData, putApiData, readJSON } from "./e2e-api";
import { loginPageViaApi } from "./e2e-auth";
import { expectedRefusalRules } from "./e2e-browser-issues";
import { isRealInstance, requireMockApi } from "./e2e-env";

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
  await expect(page.getByRole("button", { name: "Save settings" })).toBeDisabled();
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
  await page.getByRole("button", { name: "Save settings" }).click();
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

// Refused on any host: /dev/null is not a directory, so the server's
// GetOrCreateDir cannot create anything under it (nor will the mock).
const UNCREATABLE_DIR = "/dev/null/static";

const GENERAL_SAVE_REFUSAL = expectedRefusalRules({
  method: "PUT",
  path: "/api/settings/general",
  status: 400,
});

async function fillRefusedSave(page: Page, clientId: string) {
  await page.getByRole("textbox", { name: "Spotify client ID" }).fill(clientId);
  await page.getByRole("textbox", { name: "Static directory" }).fill(UNCREATABLE_DIR);
}

async function expectRefusalKeepsEdits(
  page: Page,
  baseline: GeneralSettingsType,
  clientId: string,
) {
  const staticDir = page.getByRole("textbox", { name: "Static directory" });
  await expect(page.getByText("Failed to save settings")).toBeVisible();
  await expect(
    page
      .getByRole("tabpanel", { name: "General" })
      .getByText("static directory is not accessible", { exact: true }),
  ).toBeVisible();
  await expect(staticDir).toHaveAttribute("aria-invalid", "true");
  // The optimistic copy is rolled back, the edits are not.
  await expect(staticDir).toHaveValue(UNCREATABLE_DIR);
  await expect(
    page.getByRole("textbox", { name: "Spotify client ID" }),
  ).toHaveValue(clientId);
  // Still unsaved against the rolled-back copy, so Save stays available.
  await expect(page.getByRole("button", { name: "Save settings" })).toBeEnabled();
  expect(await fetchGeneralSettings(page)).toEqual(baseline);
}

test.describe("General settings", () => {
  test("updates service settings accessibly and optimistically", async ({
    page,
  }) => {
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

      await page.getByRole("button", { name: "Save settings" }).click();
      await expect
        .poll(() => delayed.route !== null, { timeout: 5_000 })
        .toBe(true);

      expect(delayed.body).toMatchObject({
        spotify_client_id: nextSettings.spotify_client_id,
        spotify_client_secret: nextSettings.spotify_client_secret,
        enable_apple_lookups: nextSettings.enable_apple_lookups,
      });

      // Each tab fades out before it navigates: a second click during the
      // fade is ignored and the form never unmounts. Arriving on Account
      // proves it did, so General below mounts from the optimistic cache.
      await page.getByRole("tab", { name: "Account" }).click();
      await expect(page.getByText("Profile information")).toBeVisible();
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
      // Only a real instance keeps what the test saved; the mock resets before
      // each test, and a restore failing here would hide the test's own error.
      if (isRealInstance) {
        await putApiData(page.context().request, "/api/settings/general", baselineRequest);
      }
    }
  });

  test("asks before turning image downloads off", async ({ page }) => {
    // On a real instance the save would delete its downloaded artwork.
    requireMockApi();
    const puts: UpdateGeneralSettingsRequest[] = [];
    page.on("request", request => {
      if (
        request.method() === "PUT" &&
        new URL(request.url()).pathname === "/api/settings/general"
      ) {
        puts.push(request.postDataJSON() as UpdateGeneralSettingsRequest);
      }
    });

    await loginPageViaApi(page);
    expect((await fetchGeneralSettings(page)).download_images).toBe(true);

    await page.goto("/settings");
    const downloads = page.getByRole("switch", { name: "Download images" });
    await downloads.click();
    await expect(downloads).toHaveAttribute("aria-checked", "false");

    await page.getByRole("button", { name: "Save settings" }).click();
    const dialog = page.getByRole("alertdialog", {
      name: "Delete downloaded images?",
    });
    await expect(dialog).toBeVisible();
    await dialog.getByRole("button", { name: "Cancel" }).click();
    await expect(dialog).toBeHidden();
    expect(puts).toHaveLength(0);

    await page.getByRole("button", { name: "Save settings" }).click();
    await dialog.getByRole("button", { name: "Delete images" }).click();
    await expect(dialog).toBeHidden();
    await expect(page.getByText("Settings saved")).toBeVisible();
    expect(puts).toHaveLength(1);
    expect(puts[0].download_images).toBe(false);
    expect((await fetchGeneralSettings(page)).download_images).toBe(false);
  });

  test("keeps the edits and marks the directory the server refuses", async ({
    page,
    browserIssues,
  }) => {
    browserIssues.ignore(GENERAL_SAVE_REFUSAL);
    await loginPageViaApi(page);
    const baseline = await fetchGeneralSettings(page);
    const clientId = `playwright-refused-${Date.now()}`;

    try {
      await page.goto("/settings");
      const save = page.getByRole("button", { name: "Save settings" });
      await fillRefusedSave(page, clientId);
      await save.click();
      await expectRefusalKeepsEdits(page, baseline, clientId);

      const staticDir = page.getByRole("textbox", { name: "Static directory" });
      await staticDir.fill(baseline.static_dir);
      await expect(staticDir).not.toHaveAttribute("aria-invalid", "true");
      await save.click();
      await expect(page.getByText("Settings saved")).toBeVisible();
      expect((await fetchGeneralSettings(page)).spotify_client_id).toBe(clientId);
    } finally {
      // Only a real instance keeps what the test saved.
      if (isRealInstance) {
        await putApiData(
          page.context().request,
          "/api/settings/general",
          requestFromSettings(baseline),
        );
      }
    }
  });

  // The form that sent the save unmounts; the one mounted on return holds
  // the optimistic copy as its saved state when the refusal lands.
  test("keeps the edits when the refusal lands after a tab round trip", async ({
    page,
    browserIssues,
  }) => {
    browserIssues.ignore(GENERAL_SAVE_REFUSAL);
    await loginPageViaApi(page);
    const baseline = await fetchGeneralSettings(page);
    const clientId = `playwright-held-${Date.now()}`;
    const held = { route: null as Route | null };
    await page.route("**/api/settings/general", async route => {
      if (route.request().method() === "PUT" && held.route === null) {
        held.route = route;
        return;
      }
      await route.fallback();
    });

    try {
      await page.goto("/settings");
      await fillRefusedSave(page, clientId);
      await page.getByRole("button", { name: "Save settings" }).click();
      await expect.poll(() => held.route !== null).toBe(true);

      await page.getByRole("tab", { name: "Account" }).click();
      await expect(page.getByText("Profile information")).toBeVisible();
      await page.getByRole("tab", { name: "General" }).click();
      await expect(
        page.getByRole("textbox", { name: "Static directory" }),
      ).toHaveValue(UNCREATABLE_DIR);

      const refusal = page.waitForResponse(
        response =>
          new URL(response.url()).pathname === "/api/settings/general" &&
          response.request().method() === "PUT",
      );
      const route = held.route!;
      await route.continue();
      expect((await refusal).status()).toBe(400);
      await expectRefusalKeepsEdits(page, baseline, clientId);

      // A refusal from before a form mounted is not that form's: the next
      // visit shows the saved settings, unmarked.
      await page.getByRole("tab", { name: "Account" }).click();
      await expect(page.getByText("Profile information")).toBeVisible();
      await page.getByRole("tab", { name: "General" }).click();
      const staticDir = page.getByRole("textbox", { name: "Static directory" });
      await expect(staticDir).toHaveValue(baseline.static_dir);
      await expect(staticDir).not.toHaveAttribute("aria-invalid", "true");
    } finally {
      await page.unroute("**/api/settings/general").catch(() => undefined);
    }
  });
});
