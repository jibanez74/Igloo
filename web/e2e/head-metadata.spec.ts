import { expect, test, type APIResponse, type Page } from "@playwright/test";

type ApiResponse<T> = {
  error: boolean;
  message?: string;
  data?: T;
};

type HeadMetadataEnv = {
  baseURL: string;
  email: string;
  password: string;
};

const BOOTSTRAP_DESCRIPTION =
  "Igloo is your personal media center for movies, TV Shows, music, personal videos, photos and so much more. Stream and organize your entire media library.";
const MOVIES_DESCRIPTION =
  "Browse and organize your personal movie collection in your Igloo media library.";
const SETTINGS_DESCRIPTION =
  "Configure your Igloo media center settings and preferences.";

function readHeadMetadataEnv(): HeadMetadataEnv {
  return {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3000",
    email: process.env.E2E_ADMIN_EMAIL ?? "admin@example.com",
    password: process.env.E2E_ADMIN_PASSWORD ?? "AdminPassword",
  };
}

function apiURL(env: HeadMetadataEnv, path: string) {
  return new URL(path, env.baseURL).toString();
}

async function readJSON<T>(response: APIResponse) {
  return (await response.json()) as ApiResponse<T>;
}

async function login(page: Page, env: HeadMetadataEnv) {
  const loginResponse = await page.context().request.post(
    apiURL(env, "/api/auth/login"),
    {
      data: {
        email: env.email,
        password: env.password,
      },
      failOnStatusCode: false,
    },
  );
  expect(loginResponse.status()).toBe(200);

  const loginBody = await readJSON<unknown>(loginResponse);
  expect(loginBody.error, loginBody.message).toBe(false);
}

async function readActiveHeadMetadata(page: Page) {
  return page.evaluate(() => ({
    description:
      document
        .querySelector('meta[name="description"]')
        ?.getAttribute("content") ?? null,
    title: document.title,
  }));
}

test("restores bootstrap metadata on routes without page-specific head tags", async ({
  page,
}) => {
  const env = readHeadMetadataEnv();

  await login(page, env);
  await page.goto(apiURL(env, "/movies"), { waitUntil: "networkidle" });

  await expect(page).toHaveTitle("Movies - Igloo");
  await expect
    .poll(() => readActiveHeadMetadata(page))
    .toEqual({
      description: MOVIES_DESCRIPTION,
      title: "Movies - Igloo",
    });

  const playLink = page.getByRole("link", { name: /^Play / }).first();
  await expect(playLink).toBeAttached();
  await playLink.evaluate((link: HTMLAnchorElement) => {
    link.click();
  });

  await expect(page).toHaveURL(/\/movies\/\d+\/play(\?|$)/);
  await expect
    .poll(() => readActiveHeadMetadata(page))
    .toEqual({
      description: BOOTSTRAP_DESCRIPTION,
      title: "Igloo",
    });

  await page.getByRole("link", { name: "Settings" }).click();

  await expect(page).toHaveURL(/\/settings$/);
  await expect
    .poll(() => readActiveHeadMetadata(page))
    .toEqual({
      description: SETTINGS_DESCRIPTION,
      title: "Settings - Igloo",
    });
});
