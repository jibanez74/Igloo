import { expect, type APIRequestContext, type Page } from "@playwright/test";
import { readJSON } from "./e2e-api";
import { apiURL, type E2EEnv } from "./e2e-env";

// One session helper for every spec. Specs previously each carried their own
// `login`, differing only in which of the three optional checks they made, so
// those are options rather than separate functions.

type LoginOptions = {
  email?: string;
  password?: string;
  /** Assert the login envelope reports success, not just HTTP 200. */
  assertBody?: boolean;
  /** Follow up with GET /api/auth/user and assert the session took. */
  verifyUser?: boolean;
};

/** Logs in over the API, seeding the context's session cookie. */
export async function loginViaApi(
  request: APIRequestContext,
  env: E2EEnv,
  options: LoginOptions = {},
) {
  const {
    email = env.email,
    password = env.password,
    assertBody = true,
    verifyUser = false,
  } = options;

  const response = await request.post(apiURL(env, "/api/auth/login"), {
    data: { email, password },
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(200);

  if (assertBody) {
    const body = await readJSON<unknown>(response);
    expect(body.error, body.message).toBe(false);
  }

  if (verifyUser) {
    const authResponse = await request.get(apiURL(env, "/api/auth/user"), {
      failOnStatusCode: false,
    });
    expect(authResponse.status()).toBe(200);
  }
}

/** Same, for specs that hold a Page rather than an APIRequestContext. */
export async function loginPageViaApi(
  page: Page,
  env: E2EEnv,
  options: LoginOptions = {},
) {
  await loginViaApi(page.context().request, env, options);
}

export async function logoutViaApi(request: APIRequestContext, env: E2EEnv) {
  await request.delete(apiURL(env, "/api/auth/logout"), {
    failOnStatusCode: false,
  });
}
