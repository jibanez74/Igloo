import { expect, type APIRequestContext, type Page } from "@playwright/test";
import { readJSON } from "./e2e-api";
import { readE2EEnv, type Credentials } from "./e2e-env";

// One session helper for every spec. Paths resolve against the context's
// baseURL, which is the configured instance or the mocked Vite server.

/** Logs in over the API, seeding the context's session cookie. */
export async function loginViaApi(
  request: APIRequestContext,
  { email, password }: Credentials = readE2EEnv(),
) {
  const response = await request.post("/api/auth/login", {
    data: { email, password },
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(200);

  const body = await readJSON<unknown>(response);
  expect(body.error, body.message).toBe(false);
}

/** Same, for specs that hold a Page rather than an APIRequestContext. */
export async function loginPageViaApi(page: Page, credentials?: Credentials) {
  await loginViaApi(page.context().request, credentials);
}

export async function logoutViaApi(request: APIRequestContext) {
  await request.delete("/api/auth/logout", { failOnStatusCode: false });
}

/**
 * Fills in and sends the sign-in form, returning the login response.
 */
export async function submitLogin(page: Page, email: string, password: string) {
  // After a sign-out the address changes a moment before the page does:
  // until the sign-in form is up, the account page's own email fields still
  // answer to "Email".
  await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password", { exact: true }).fill(password);

  const loginResponsePromise = page.waitForResponse(
    response =>
      response.url().includes("/api/auth/login") &&
      response.request().method() === "POST",
  );

  await page.getByRole("button", { name: "Sign in" }).click();

  return loginResponsePromise;
}
