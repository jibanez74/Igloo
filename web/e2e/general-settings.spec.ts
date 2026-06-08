import { expect, test, type APIResponse, type Page } from "@playwright/test";

import { apiURL, readE2EEnv, type E2EEnv } from "./e2e-env";
import { isIgnorableFailedRequest } from "./e2e-browser-issues";

type ApiResponse<T> = {
  error: boolean;
  message?: string;
  data?: T;
};

type HardwareAccelerationDevice = "cpu" | "apple" | "nvidia" | "intel";

type GeneralSettings = {
  tmdb_key: string | null;
  immich_base_url: string | null;
  immich_api_key: string | null;
  jellyfin_base_url: string | null;
  jellyfin_api_key: string | null;
  spotify_client_id: string | null;
  spotify_client_secret: string | null;
  hardware_acceleration_device: HardwareAccelerationDevice;
  enable_logger: boolean;
  enable_watcher: boolean;
  download_images: boolean;
  static_dir: string;
  logs_dir: string;
  transcode_dir: string;
  server_upload_mbps: number | null;
};

type GeneralSettingsData = {
  settings: GeneralSettings;
};

type GeneralSettingsRequest = {
  tmdb_key: string;
  immich_base_url: string;
  immich_api_key: string;
  jellyfin_base_url: string;
  jellyfin_api_key: string;
  spotify_client_id: string;
  spotify_client_secret: string;
  hardware_acceleration_device: HardwareAccelerationDevice;
  enable_logger: boolean;
  enable_watcher: boolean;
  download_images: boolean;
  static_dir: string;
  logs_dir: string;
  transcode_dir: string;
  server_upload_mbps: number | null;
};

function requestFromSettings(settings: GeneralSettings): GeneralSettingsRequest {
  return {
    tmdb_key: settings.tmdb_key ?? "",
    immich_base_url: settings.immich_base_url ?? "",
    immich_api_key: settings.immich_api_key ?? "",
    jellyfin_base_url: settings.jellyfin_base_url ?? "",
    jellyfin_api_key: settings.jellyfin_api_key ?? "",
    spotify_client_id: settings.spotify_client_id ?? "",
    spotify_client_secret: settings.spotify_client_secret ?? "",
    hardware_acceleration_device: settings.hardware_acceleration_device,
    enable_logger: settings.enable_logger,
    enable_watcher: settings.enable_watcher,
    download_images: settings.download_images,
    static_dir: settings.static_dir,
    logs_dir: settings.logs_dir,
    transcode_dir: settings.transcode_dir,
    server_upload_mbps: settings.server_upload_mbps ?? null,
  };
}

async function readJSON<T>(response: APIResponse) {
  return (await response.json()) as ApiResponse<T>;
}

async function expectAPIData<T>(response: APIResponse, expectedStatus: number) {
  expect(response.status()).toBe(expectedStatus);

  const body = await readJSON<T>(response);
  expect(body.error, body.message).toBe(false);
  expect(body.data).toBeTruthy();
  return body.data!;
}

async function login(page: Page, env: E2EEnv) {
  const loginResponse = await page.context().request.post(apiURL(env, "/api/auth/login"), {
    data: {
      email: env.email,
      password: env.password,
    },
    failOnStatusCode: false,
  });
  expect(loginResponse.status()).toBe(200);

  const loginBody = await readJSON<unknown>(loginResponse);
  expect(loginBody.error, loginBody.message).toBe(false);

  const authResponse = await page.context().request.get(apiURL(env, "/api/auth/user"), {
    failOnStatusCode: false,
  });
  expect(authResponse.status()).toBe(200);
}

async function fetchGeneralSettings(page: Page, env: E2EEnv) {
  const response = await page.context().request.get(
    apiURL(env, "/api/settings/general"),
    { failOnStatusCode: false },
  );

  const data = await expectAPIData<GeneralSettingsData>(response, 200);
  return data.settings;
}

async function restoreGeneralSettings(
  page: Page,
  env: E2EEnv,
  settings: GeneralSettingsRequest,
) {
  const response = await page.context().request.put(
    apiURL(env, "/api/settings/general"),
    {
      data: settings,
      failOnStatusCode: false,
    },
  );
  expect(response.status()).toBe(200);

  const body = await readJSON<GeneralSettingsData>(response);
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

async function activeElementName(page: Page) {
  return page.evaluate(() => {
    const active = document.activeElement;
    if (!(active instanceof HTMLElement)) {
      return null;
    }

    const ariaLabel = active.getAttribute("aria-label");
    if (ariaLabel) {
      return ariaLabel;
    }

    if (active.id) {
      const label = document.querySelector(
        `label[for="${CSS.escape(active.id)}"]`,
      );
      if (label?.textContent) {
        return label.textContent.trim();
      }
    }

    return (
      active.textContent?.trim() ||
      active.getAttribute("name") ||
      active.tagName
    );
  });
}

async function expectNewIntegrationControls(page: Page) {
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

async function expectDescribedByContains(
  page: Page,
  name: string,
  expectedDescription: string,
) {
  const input = page.getByRole("textbox", { name });
  const descriptionId = await input.getAttribute("aria-describedby");
  expect(descriptionId, `${name} should have aria-describedby`).toBeTruthy();

  await expect(page.locator(`#${descriptionId}`)).toContainText(
    expectedDescription,
  );
}

async function expectScreenReaderSupport(page: Page) {
  await expectDescribedByContains(
    page,
    "Jellyfin base URL",
    "http:// or https://",
  );
  await expectDescribedByContains(
    page,
    "Immich base URL",
    "http:// or https://",
  );
  await expectDescribedByContains(
    page,
    "Jellyfin API key",
    "Leave blank to clear",
  );
  await expectDescribedByContains(
    page,
    "Immich API key",
    "Leave blank to clear",
  );

  await page.getByRole("textbox", { name: "Jellyfin base URL" }).fill(
    "ftp://not-valid.local",
  );
  await page.getByRole("button", { name: "Save Settings" }).click();
  await expect(
    page.locator('p[aria-live="polite"]').filter({
      hasText: "Jellyfin base URL must start",
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("textbox", { name: "Jellyfin base URL" }),
  ).toHaveAttribute("aria-invalid", "true");
  await expect(page.locator('p[aria-live="polite"]')).toContainText(
    "Jellyfin base URL must start",
  );
  await expect.poll(() => activeElementName(page)).not.toBeNull();
}

async function expectKeyboardFlow(page: Page) {
  await page.getByRole("textbox", { name: "TMDB API key" }).focus();

  for (const expectedName of [
    "Show TMDB API key",
    "Jellyfin base URL",
    "Jellyfin API key",
    "Show Jellyfin API key",
    "Immich base URL",
    "Immich API key",
    "Show Immich API key",
  ]) {
    await page.keyboard.press("Tab");
    await expect.poll(() => activeElementName(page)).toBe(expectedName);
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
    GeneralSettingsRequest,
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

test.describe.configure({ mode: "serial" });

test.describe("General settings", () => {
  test("updates integration settings accessibly and optimistically", async ({
    page,
  }) => {
    const env = readE2EEnv();
    const consoleErrors: string[] = [];
    const pageErrors: string[] = [];
    const failedRequests: string[] = [];
    const settingsErrors: string[] = [];

    page.on("console", message => {
      if (message.type() === "error") {
        consoleErrors.push(message.text());
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
      if (response.url().includes("/api/settings/general") && response.status() >= 400) {
        settingsErrors.push(
          `${response.status()} ${response.request().method()} ${response.url()}`,
        );
      }
    });

    await login(page, env);
    const baselineSettings = await fetchGeneralSettings(page, env);
    const baselineRequest = requestFromSettings(baselineSettings);
    const stamp = Date.now();
    const nextSettings: GeneralSettingsRequest = {
      ...baselineRequest,
      jellyfin_base_url: `https://jellyfin-playwright-${stamp}.local:8096`,
      jellyfin_api_key: `playwright-jellyfin-key-${stamp}`,
      immich_base_url: `http://immich-playwright-${stamp}.local:2283`,
      immich_api_key: `playwright-immich-key-${stamp}`,
    };

    let delayedRoute:
      | Parameters<Parameters<typeof page.route>[1]>[0]
      | null = null;

    try {
      await page.goto(apiURL(env, "/settings"), { waitUntil: "networkidle" });
      await expectNewIntegrationControls(page);
      await expectScreenReaderSupport(page);
      await expectKeyboardFlow(page);
      await fillIntegrationSettings(page, nextSettings);

      let delayedBody: GeneralSettingsRequest | null = null;
      await page.route("**/api/settings/general", async route => {
        const request = route.request();
        if (request.method() === "PUT" && delayedRoute === null) {
          delayedRoute = route;
          delayedBody = request.postDataJSON() as GeneralSettingsRequest;
          return;
        }

        await route.continue();
      });

      await page.getByRole("button", { name: "Save Settings" }).click();
      await expect
        .poll(() => delayedRoute !== null, { timeout: 5_000 })
        .toBe(true);

      expect(delayedBody?.jellyfin_base_url).toBe(
        nextSettings.jellyfin_base_url,
      );
      expect(delayedBody?.jellyfin_api_key).toBe(
        nextSettings.jellyfin_api_key,
      );
      expect(delayedBody?.immich_base_url).toBe(nextSettings.immich_base_url);
      expect(delayedBody?.immich_api_key).toBe(nextSettings.immich_api_key);

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
      await delayedRoute?.continue();
      delayedRoute = null;

      const putResponse = await putResponsePromise;
      expect(putResponse.status()).toBe(200);
      const putBody = await readJSON<GeneralSettingsData>(putResponse);
      expect(putBody.error, putBody.message).toBe(false);

      await page.unroute("**/api/settings/general");
      await expect(page.getByText("Settings saved").first()).toBeVisible();

      const savedSettings = await fetchGeneralSettings(page, env);
      expect(savedSettings.jellyfin_base_url).toBe(
        nextSettings.jellyfin_base_url,
      );
      expect(savedSettings.jellyfin_api_key).toBe(
        nextSettings.jellyfin_api_key,
      );
      expect(savedSettings.immich_base_url).toBe(nextSettings.immich_base_url);
      expect(savedSettings.immich_api_key).toBe(nextSettings.immich_api_key);
    } finally {
      if (delayedRoute) {
        await delayedRoute.continue().catch(() => undefined);
      }
      await page.unroute("**/api/settings/general").catch(() => undefined);
      await restoreGeneralSettings(page, env, baselineRequest);
      await page.goto(apiURL(env, "/settings"), { waitUntil: "networkidle" });
    }

    await expect
      .poll(() => integrationFieldValues(page))
      .toMatchObject({
        jellyfin_base_url: baselineRequest.jellyfin_base_url,
        jellyfin_api_key: baselineRequest.jellyfin_api_key,
        immich_base_url: baselineRequest.immich_base_url,
        immich_api_key: baselineRequest.immich_api_key,
      });

    expect(consoleErrors).toEqual([]);
    expect(pageErrors).toEqual([]);
    expect(failedRequests).toEqual([]);
    expect(settingsErrors).toEqual([]);
  });
});
