import { expect, test, type APIRequestContext, type Page } from "@playwright/test";
import { readJSON } from "./e2e-api";
import { isRealInstance, readE2EEnv, type Credentials } from "./e2e-env";

// One session helper for every spec. Paths resolve against the context's
// baseURL, which is the configured instance or the mocked Vite server.

// A real instance takes 10 sign-ins per 5 minutes from one IP
// (loginAttemptLimit in server/cmd/api/auth_handler.go), fewer than one
// real-instance run makes. A refused attempt does not use up the budget, so
// the helpers below try again until the window rolls over, stretching the
// running test's timeout to cover the wait. The mock never refuses a sign-in.
const LOGIN_WINDOW_MS = 5 * 60_000;
const LOGIN_RETRY_MS = 10_000;

/** Sends a sign-in until a real instance stops refusing it for the limit. */
async function outlastLoginLimit<T extends { status(): number }>(
  attempt: () => Promise<T>,
): Promise<T> {
  let response = await attempt();
  if (!isRealInstance || response.status() !== 429) return response;

  // Enough for the longest wait: a window that has only just started.
  const waitBudgetMs = LOGIN_WINDOW_MS + LOGIN_RETRY_MS;
  const info = test.info();
  if (info.timeout > 0) info.setTimeout(info.timeout + waitBudgetMs);
  const deadline = Date.now() + waitBudgetMs;
  while (response.status() === 429 && Date.now() < deadline) {
    await new Promise(resolve => setTimeout(resolve, LOGIN_RETRY_MS));
    response = await attempt();
  }
  return response;
}

/** Logs in over the API, seeding the context's session cookie. */
export async function loginViaApi(
  request: APIRequestContext,
  { email, password }: Credentials = readE2EEnv(),
) {
  const response = await outlastLoginLimit(() =>
    request.post("/api/auth/login", {
      data: { email, password },
      failOnStatusCode: false,
    }),
  );
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

  return outlastLoginLimit(async () => {
    const loginResponsePromise = page.waitForResponse(
      response =>
        response.url().includes("/api/auth/login") &&
        response.request().method() === "POST",
    );

    await page.getByRole("button", { name: "Sign in" }).click();

    return loginResponsePromise;
  });
}
