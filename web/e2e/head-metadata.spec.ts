import { expect, test, type APIResponse, type Page } from "@playwright/test";

import { apiURL, readE2EEnv, type E2EEnv } from "./e2e-env";

type ApiResponse<T> = {
  error: boolean;
  message?: string;
  data?: T;
};

const BOOTSTRAP_DESCRIPTION =
  "Igloo is your personal media center for movies, TV Shows, music, personal videos, photos and so much more. Stream and organize your entire media library.";
const MOVIES_DESCRIPTION =
  "Browse and organize your personal movie collection in your Igloo media library.";
const SETTINGS_DESCRIPTION =
  "Configure your Igloo media center settings and preferences.";

async function readJSON<T>(response: APIResponse) {
  return (await response.json()) as ApiResponse<T>;
}

async function login(page: Page, env: E2EEnv) {
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
  const env = readE2EEnv();

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
