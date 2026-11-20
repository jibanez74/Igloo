import { expect, test, type BrowserIssueOptions } from "./e2e-test";
import type { BrowserContext, Page, Response } from "@playwright/test";

import { readE2EEnv, requireRealInstance } from "./e2e-env";
import { isApiResourceEcho, isSessionEndedResponse } from "./e2e-browser-issues";
import { readJSON } from "./e2e-api";
import { loginPageViaApi, submitLogin } from "./e2e-auth";
import { auditResponsiveControls } from "./e2e-layout";

function isExpectedLoggedOutAuthResponse(response: Response) {
  return response.status() === 401 && response.url().includes("/api/auth/user");
}

function isExpectedInvalidLoginResponse(response: Response) {
  return (
    response.status() === 401 &&
    response.url().includes("/api/auth/login") &&
    response.request().method() === "POST"
  );
}

// Only a real instance without a TMDB key answers this 503; the mock always
// serves in-theaters data.
function isExpectedInTheatersUnavailableResponse(response: Response) {
  const url = new URL(response.url());

  return (
    response.status() === 503 &&
    response.request().method() === "GET" &&
    url.pathname === "/api/tmdb/movies/in-theaters"
  );
}

/** What every login test drives on purpose; a test adds its own with `browserIssues.ignore`. */
const loginIssueRules: BrowserIssueOptions = {
  // ignoreResponse judges every failed /api/ response; the browser's console
  // echo of each one adds nothing.
  ignoreConsole: (_type, text, message) => isApiResourceEcho(text, message),
  ignoreResponse: response =>
    isExpectedLoggedOutAuthResponse(response) ||
    isExpectedInTheatersUnavailableResponse(response),
};

/** The URL is `pathname` on the app's own origin, whatever its query. */
function appPath(pathname: string) {
  const origin = new URL(readE2EEnv().baseURL).origin;
  return (url: URL) => url.origin === origin && url.pathname === pathname;
}

async function expectUnauthenticated(context: BrowserContext) {
  const response = await context.request.get("/api/auth/user", {
    failOnStatusCode: false,
  });

  expect(response.status()).toBe(401);

  const body = await readJSON<unknown>(response);
  expect(body.error, body.message).toBe(true);
}

async function expectAuthenticated(context: BrowserContext) {
  const response = await context.request.get("/api/auth/user", {
    failOnStatusCode: false,
  });

  expect(response.status()).toBe(200);

  const body = await readJSON<unknown>(response);
  expect(body.error, body.message).toBe(false);
}

async function expectLoginControls(page: Page) {
  await expect(page.getByLabel("Email")).toBeVisible();
  await expect(page.getByLabel("Password", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Show password" }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
}

test.describe("Login screen", () => {
  test.use({ browserIssueOptions: loginIssueRules });

  // What needs a real browser: where focus lands on load, the tab order, and
  // that the browser's own validation stays out of the way. The messages,
  // the password toggle and the field attributes are unit tests
  // (src/test/app/login-route.test.tsx).
  test("focuses the email, tabs through the form, and validates an empty one itself", async ({
    page,
    context,
  }) => {
    let loginRequests = 0;

    page.on("request", request => {
      if (
        request.url().includes("/api/auth/login") &&
        request.method() === "POST"
      ) {
        loginRequests += 1;
      }
    });

    await page.goto("/login");
    await expectLoginControls(page);

    const email = page.getByLabel("Email");
    const password = page.getByLabel("Password", { exact: true });

    await expect(email).toBeFocused();
    await page.keyboard.press("Tab");
    await expect(password).toBeFocused();
    await page.keyboard.press("Tab");
    await expect(
      page.getByRole("button", { name: "Show password" }),
    ).toBeFocused();
    await page.keyboard.press("Tab");
    await expect(page.getByRole("button", { name: "Sign in" })).toBeFocused();

    await page.keyboard.press("Enter");

    // Inline, app-styled validation rather than the browser's bubble (§3.7):
    // a form the browser validated would never reach the app's handler.
    await expect(page.getByText("Email is required.")).toBeVisible();
    await expect(page.getByText("Password is required.")).toBeVisible();
    await expect(email).toBeFocused();
    await expect(page).toHaveURL(appPath("/login"));

    await expectUnauthenticated(context);
    expect(loginRequests, "an invalid form should not submit").toBe(0);
  });

  test("shows login failure without creating a session", async ({
    page,
    context,
    browserIssues,
  }) => {
    requireRealInstance("the mock server's credential check is its own code");
    const env = readE2EEnv();
    browserIssues.ignore({ ignoreResponse: isExpectedInvalidLoginResponse });

    await page.goto("/login");
    await expectLoginControls(page);

    const loginResponse = await submitLogin(
      page,
      env.email,
      "WrongPassword",
    );
    expect(loginResponse.status()).toBe(401);

    const loginBody = await readJSON<unknown>(loginResponse);
    expect(loginBody.error, loginBody.message).toBe(true);

    await expect(page).toHaveURL(appPath("/login"));
    await expect(
      page
        .getByRole("region", { name: /^Notifications/ })
        .getByText("Login failed"),
    ).toBeVisible();
    await expect(page.getByLabel("Email")).toBeEnabled();
    await expect(page.getByLabel("Password", { exact: true })).toBeEnabled();
    await expect(page.getByRole("button", { name: "Sign in" })).toBeEnabled();
    await expectUnauthenticated(context);
  });

  test("signs in through the UI, follows a safe redirect, and skips the form once signed in", async ({
    page,
    context,
  }) => {
    const env = readE2EEnv();
    await page.goto("/login?redirect=/settings/account");
    await expectLoginControls(page);

    const loginResponse = await submitLogin(page, env.email, env.password);
    expect(loginResponse.status()).toBe(200);

    const loginBody = await readJSON<unknown>(loginResponse);
    expect(loginBody.error, loginBody.message).toBe(false);

    await expect(page).toHaveURL(appPath("/settings/account"));
    await expect(page.getByText("Profile information")).toBeVisible();
    await expectAuthenticated(context);

    await page.goto("/login?redirect=/settings/account");
    await expect(page).toHaveURL(appPath("/settings/account"));
    await expect(page.getByText("Profile information")).toBeVisible();
  });

  test("logs out through the sidebar and blocks protected routes", async ({
    page,
    context,
  }) => {
    await loginPageViaApi(page);
    await page.goto("/settings/account");
    await expect(page.getByText("Profile information")).toBeVisible();

    const logoutResponsePromise = page.waitForResponse(
      response =>
        response.url().includes("/api/auth/logout") &&
        response.request().method() === "DELETE",
    );

    // Logout lives in the account menu at the foot of the sidebar.
    await page.getByRole("button", { name: /^Account: / }).click();
    await page.getByRole("menuitem", { name: "Logout" }).click();

    const logoutResponse = await logoutResponsePromise;
    expect(logoutResponse.status()).toBe(200);

    const logoutBody = await readJSON<unknown>(logoutResponse);
    expect(logoutBody.error, logoutBody.message).toBe(false);

    // Logout itself leaves for the sign-in page, which then leads Home (the
    // search's default). A shell that stayed put would get there anyway,
    // through the 401s of its next requests, but as an expired session: back
    // to this page, with a notice.
    await expect(page).toHaveURL(appPath("/login"));
    await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
    expect(new URL(page.url()).searchParams.get("redirect")).toBe("/");
    await expect(page.getByText("You've been signed out.")).toHaveCount(0);
    await expectUnauthenticated(context);

    await page.goto("/settings/account");
    await expect(page).toHaveURL(appPath("/login"));
    await expect
      .poll(() => new URL(page.url()).searchParams.get("redirect"))
      .toBe("/settings/account");
  });

  // Clearing the cookie stands in for the server ending the session (expiry,
  // a password change elsewhere): every request after it is a 401.
  test("sends a reader whose session ended on a navigation to log in, then back", async ({
    page,
    context,
    browserIssues,
  }) => {
    const env = readE2EEnv();
    browserIssues.ignore({ ignoreResponse: isSessionEndedResponse });

    await loginPageViaApi(page);
    await page.goto("/settings/account");
    await expect(page.getByText("Profile information")).toBeVisible();

    await context.clearCookies();
    await page.getByRole("link", { name: "Movies" }).click();

    await expect(page).toHaveURL(appPath("/login"));
    await expect
      .poll(() => new URL(page.url()).searchParams.get("redirect"))
      .toBe("/movies");
    await expect(page.getByText("You've been signed out.")).toBeVisible();

    const loginResponse = await submitLogin(page, env.email, env.password);
    expect(loginResponse.status()).toBe(200);
    await expect(page).toHaveURL(appPath("/movies"));
  });

  test("signs out when a request on the open page hears the session ended", async ({
    page,
    context,
    browserIssues,
  }) => {
    const env = readE2EEnv();
    browserIssues.ignore({ ignoreResponse: isSessionEndedResponse });

    await loginPageViaApi(page);
    await page.goto("/settings/account");
    await expect(page.getByText("Profile information")).toBeVisible();

    await context.clearCookies();
    // Opening the bell fetches the notification list, with no navigation.
    await page.getByRole("button", { name: /^Notifications/ }).click();

    await expect(page).toHaveURL(appPath("/login"));
    await expect
      .poll(() => new URL(page.url()).searchParams.get("redirect"))
      .toBe("/settings/account");
    await submitLogin(page, env.email, env.password);
    await expect(page).toHaveURL(appPath("/settings/account"));
    await expect(page.getByText("Profile information")).toBeVisible();
  });

  test("is responsive and keeps screen reader support at each viewport", async ({
    page,
  }) => {
    await auditResponsiveControls(page, {
      ready: async () => {
        await page.goto("/login");
        await expectLoginControls(page);
      },
      minFontPx: 12,
    });
  });
});
