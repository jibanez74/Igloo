import {
  expect,
  test,
  type APIResponse,
  type Locator,
  type Page,
  type Response,
} from "@playwright/test";

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

type PlaybackSettings = {
  profiles: PlaybackProfile[];
  preferred_profile: string | null;
  download_mbps: number | null;
  server_upload_mbps: number | null;
  is_admin: boolean;
  preferred_audio_language: string | null;
  preferred_subtitle_language: string | null;
};

type PlaybackSettingsData = {
  settings: PlaybackSettings;
};

type UpdatePlaybackSettingsRequest = {
  preferred_profile: string | null;
  download_mbps: number | null;
  preferred_audio_language: string | null;
  preferred_subtitle_language: string | null;
  server_upload_mbps?: number | null;
};

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

type PlaybackSettingsEnv = {
  baseURL: string;
  email: string;
  password: string;
};

const DOWNLOAD_SPEED_VALIDATION_MESSAGE =
  "Download speed must be between 0 and 10000 Mbps.";

function readPlaybackSettingsEnv(): PlaybackSettingsEnv {
  return {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3000",
    email: process.env.E2E_ADMIN_EMAIL ?? "admin@sample.com",
    password: process.env.E2E_ADMIN_PASSWORD ?? "AdminPassword",
  };
}

function apiURL(env: PlaybackSettingsEnv, path: string) {
  return new URL(path, env.baseURL).toString();
}

async function readJSON<T>(response: APIResponse | Response) {
  return (await response.json()) as ApiResponse<T>;
}

async function login(
  page: Page,
  env: PlaybackSettingsEnv,
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

async function logout(page: Page, env: PlaybackSettingsEnv) {
  await page.context().request.delete(apiURL(env, "/api/auth/logout"), {
    failOnStatusCode: false,
  });
}

async function fetchPlaybackSettings(page: Page, env: PlaybackSettingsEnv) {
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
  env: PlaybackSettingsEnv,
  settings: PlaybackSettings,
) {
  const response = await page.context().request.put(
    apiURL(env, "/api/settings/playback"),
    {
      data: {
        preferred_profile: settings.preferred_profile,
        download_mbps: settings.download_mbps,
        preferred_audio_language: settings.preferred_audio_language,
        preferred_subtitle_language: settings.preferred_subtitle_language,
        server_upload_mbps: settings.server_upload_mbps,
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
  env: PlaybackSettingsEnv,
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

async function deleteUser(page: Page, env: PlaybackSettingsEnv, userId: number) {
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

test.describe.configure({ mode: "serial" });

test.describe("Playback settings", () => {
  test("lets admins save personal playback settings and server bandwidth together", async ({
    page,
  }) => {
    const env = readPlaybackSettingsEnv();
    const tracker = trackBrowserIssues(page);
    let putBody: UpdatePlaybackSettingsRequest | null = null;

    page.on("request", request => {
      const url = new URL(request.url());
      if (
        url.pathname === "/api/settings/playback" &&
        request.method() === "PUT"
      ) {
        putBody = request.postDataJSON() as UpdatePlaybackSettingsRequest;
      }
    });

    await login(page, env);
    const baselineSettings = await fetchPlaybackSettings(page, env);

    try {
      await page.goto(apiURL(env, "/settings/playback"), {
        waitUntil: "networkidle",
      });
      await expect(
        page.getByRole("heading", { name: "Playback Settings" }),
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

      await downloadInput.fill("100");
      await serverInput.fill("5");
      await expect(page.getByText("Recommended: 1080p · 4 Mbps")).toBeVisible();

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
      expect(putBody?.download_mbps).toBe(100);
      expect(putBody?.server_upload_mbps).toBe(5);

      await expect(page.getByText("Playback settings saved")).toBeVisible();
      const savedSettings = await fetchPlaybackSettings(page, env);
      expect(savedSettings.download_mbps).toBe(100);
      expect(savedSettings.server_upload_mbps).toBe(5);
    } finally {
      await restorePlaybackSettings(page, env, baselineSettings);
    }

    tracker.assertClean();
  });

  test("validates zero download speed before saving", async ({ page }) => {
    const env = readPlaybackSettingsEnv();
    const tracker = trackBrowserIssues(page);

    await login(page, env);

    await page.goto(apiURL(env, "/settings/playback"), {
      waitUntil: "networkidle",
    });
    const downloadInput = page.getByRole("spinbutton", {
      name: "Download speed (Mbps)",
    });

    await downloadInput.fill("0");
    const successfulPutPromise = page
      .waitForResponse(
        response => {
          const url = new URL(response.url());
          return (
            url.pathname === "/api/settings/playback" &&
            response.request().method() === "PUT" &&
            response.status() < 400
          );
        },
        { timeout: 500 },
      )
      .then(() => true)
      .catch(() => false);
    await page.getByRole("button", { name: "Save Settings" }).click();

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
    expect(await successfulPutPromise).toBe(false);

    tracker.assertClean();
  });

  test("keeps server bandwidth read-only for regular users", async ({ page }) => {
    const env = readPlaybackSettingsEnv();
    const stamp = Date.now();
    const tracker = trackBrowserIssues(page);
    let regularUser: AdminUser | null = null;
    let regularPassword = "";
    let putBody: UpdatePlaybackSettingsRequest | null = null;

    page.on("request", request => {
      const url = new URL(request.url());
      if (
        url.pathname === "/api/settings/playback" &&
        request.method() === "PUT"
      ) {
        putBody = request.postDataJSON() as UpdatePlaybackSettingsRequest;
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
        page.getByText(
          "Set by the server administrator. Affects your recommendation",
        ),
      ).toBeVisible();

      await page
        .getByRole("spinbutton", { name: "Download speed (Mbps)" })
        .fill("30");

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
      expect(putBody?.download_mbps).toBe(30);
      expect(
        Object.prototype.hasOwnProperty.call(putBody ?? {}, "server_upload_mbps"),
      ).toBe(false);

      const maliciousResponse = await page.context().request.put(
        apiURL(env, "/api/settings/playback"),
        {
          data: {
            preferred_profile: null,
            download_mbps: 35,
            preferred_audio_language: null,
            preferred_subtitle_language: null,
            server_upload_mbps: 9,
          } satisfies UpdatePlaybackSettingsRequest,
          failOnStatusCode: false,
        },
      );
      expect(maliciousResponse.status()).toBe(403);
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
