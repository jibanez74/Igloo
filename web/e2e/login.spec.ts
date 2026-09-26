import {
  expect,
  test,
  type BrowserContext,
  type Page,
  type Response,
} from "@playwright/test";

import { readE2EEnv } from "./e2e-env";
import { trackBrowserIssues } from "./e2e-browser-issues";
import {
  readJSON,
} from "./e2e-api";
import { logoutViaApi } from "./e2e-auth";
import { expectPageHasNoHorizontalScroll, VIEWPORTS } from "./e2e-layout";

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

function isExpectedInTheatersUnavailableResponse(response: Response) {
  const url = new URL(response.url());

  return (
    response.status() === 503 &&
    response.request().method() === "GET" &&
    url.pathname === "/api/tmdb/movies/in-theaters"
  );
}

function trackLoginBrowserIssues(
  page: Page,
  isExpectedResponse: (response: Response) => boolean =
    isExpectedLoggedOutAuthResponse,
) {
  return trackBrowserIssues(page, {
    // ignoreResponse judges every failed response; the browser's console echo
    // of each one adds nothing.
    ignoreConsole: (_type, text) => text.startsWith("Failed to load resource:"),
    ignoreResponse: response =>
      isExpectedResponse(response) ||
      isExpectedInTheatersUnavailableResponse(response),
  });
}

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
  await expect(page).toHaveTitle("Sign In - Igloo");
  await expect(page.getByText("Welcome to Igloo")).toBeVisible();
  await expect(
    page.getByText("Sign in to access your private media library."),
  ).toBeVisible();
  await expect(page.getByLabel("Email")).toBeVisible();
  await expect(page.getByLabel("Password", { exact: true })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Show password" }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
}

async function submitLogin(page: Page, email: string, password: string) {
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

async function expectLoginLayout(page: Page) {
  const layout = await page.evaluate(() => {
    const visible = (element: Element) => {
      const style = getComputedStyle(element);
      const rect = element.getBoundingClientRect();

      return (
        style.display !== "none" &&
        style.visibility !== "hidden" &&
        rect.width > 0 &&
        rect.height > 0
      );
    };

    const accessibleName = (element: Element) => {
      const ariaLabel = element.getAttribute("aria-label");
      if (ariaLabel) {
        return ariaLabel.trim();
      }

      const labelledBy = element.getAttribute("aria-labelledby");
      if (labelledBy) {
        return labelledBy
          .split(/\s+/)
          .map(id => document.getElementById(id)?.textContent?.trim() ?? "")
          .join(" ")
          .trim();
      }

      if (element.id) {
        const label = document.querySelector(
          `label[for="${CSS.escape(element.id)}"]`,
        );
        if (label?.textContent) {
          return label.textContent.trim();
        }
      }

      const text = element.textContent?.trim();
      if (text) {
        return text;
      }

      const placeholder = element.getAttribute("placeholder");
      if (placeholder) {
        return placeholder.trim();
      }

      return "";
    };

    const interactive = Array.from(
      document.querySelectorAll(
        'button, a[href], input, select, textarea, [role="button"], [tabindex]:not([tabindex="-1"])',
      ),
    ).filter(visible);

    const overflowX = Array.from(document.querySelectorAll("body, body *"))
      .filter(visible)
      .filter(element => {
        const rect = element.getBoundingClientRect();
        return rect.left < -1 || rect.right > window.innerWidth + 1;
      })
      .map(element => ({
        tag: element.tagName.toLowerCase(),
        text: element.textContent?.trim().slice(0, 60) ?? "",
      }));

    const unlabeled = interactive
      .filter(element => !accessibleName(element))
      .map(element => ({
        tag: element.tagName.toLowerCase(),
        id: element.id,
        role: element.getAttribute("role"),
      }));

    const smallText = Array.from(
      document.querySelectorAll("label, p, span, button, a, input"),
    )
      .filter(visible)
      .map(element => ({
        text:
          element.textContent?.trim() ||
          element.getAttribute("placeholder") ||
          "",
        fontSize: Number.parseFloat(getComputedStyle(element).fontSize),
      }))
      .filter(item => item.text && item.fontSize < 12);

    const card = document.querySelector('[data-slot="card"]');
    const submit = Array.from(document.querySelectorAll("button")).find(button =>
      button.textContent?.includes("Sign in"),
    );

    return {
      overflowX,
      unlabeled,
      smallText,
      hasBackgroundImage: Boolean(
        document.querySelector('img[aria-hidden="true"][alt=""]'),
      ),
      cardClassName: card?.className.toString() ?? "",
      cardAnimationName: card ? getComputedStyle(card).animationName : "",
      submitVariant: submit?.getAttribute("data-variant") ?? "",
    };
  });

  await expectPageHasNoHorizontalScroll(page);
  expect(layout.overflowX).toEqual([]);
  expect(layout.unlabeled).toEqual([]);
  expect(layout.smallText).toEqual([]);
  expect(layout.hasBackgroundImage).toBe(true);
  expect(layout.cardClassName).toContain("animate-in");
  expect(layout.cardClassName).toContain("fade-in");
  expect(layout.cardClassName).toContain("slide-in-from-bottom-2");
  expect(layout.cardAnimationName).toBe("enter");
  expect(layout.submitVariant).toBe("accent");
}

test.describe("Login screen", () => {
  test.afterEach(async ({ context }) => {
    await logoutViaApi(context.request);
  });

  test("renders accessibly and blocks empty submissions before the API call", async ({
    page,
    context,
  }) => {
    const env = readE2EEnv();
    const tracker = trackLoginBrowserIssues(page);
    let loginRequests = 0;

    page.on("request", request => {
      if (
        request.url().includes("/api/auth/login") &&
        request.method() === "POST"
      ) {
        loginRequests += 1;
      }
    });

    await logoutViaApi(context.request);
    await page.goto("/login", { waitUntil: "networkidle" });
    await expectLoginControls(page);

    const email = page.getByLabel("Email");
    const password = page.getByLabel("Password", { exact: true });

    await expect(email).toBeFocused();
    await expect(email).toHaveAttribute("autocomplete", "username");
    await expect(password).toHaveAttribute("type", "password");

    await password.fill(env.password);
    await page.getByRole("button", { name: "Show password" }).click();
    await expect(password).toHaveAttribute("type", "text");
    await expect(
      page.getByRole("button", { name: "Hide password" }),
    ).toBeVisible();

    await page.getByRole("button", { name: "Hide password" }).click();
    await expect(password).toHaveAttribute("type", "password");

    await email.focus();
    await page.keyboard.press("Tab");
    await expect(password).toBeFocused();
    await page.keyboard.press("Tab");
    await expect(
      page.getByRole("button", { name: "Show password" }),
    ).toBeFocused();
    await page.keyboard.press("Tab");
    await expect(page.getByRole("button", { name: "Sign in" })).toBeFocused();

    await email.fill("");
    await password.fill("");
    await page.getByRole("button", { name: "Sign in" }).click();

    await expect(page).toHaveURL(appPath("/login"));
    await expect
      .poll(() => loginRequests, { message: "empty form should not submit" })
      .toBe(0);
    await expect(email).toHaveJSProperty("validity.valueMissing", true);
    await expect(password).toHaveJSProperty("validity.valueMissing", true);
    await expectUnauthenticated(context);

    tracker.assertClean();
  });

  test("shows login failure without creating a session", async ({
    page,
    context,
  }) => {
    const env = readE2EEnv();
    const tracker = trackLoginBrowserIssues(
      page,
      response =>
        isExpectedLoggedOutAuthResponse(response) ||
        isExpectedInvalidLoginResponse(response),
    );

    await logoutViaApi(context.request);
    await page.goto("/login", { waitUntil: "networkidle" });
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
    await expect(page.getByText("Login failed").first()).toBeVisible();
    await expect(page.getByLabel("Email")).toBeEnabled();
    await expect(page.getByLabel("Password", { exact: true })).toBeEnabled();
    await expect(page.getByRole("button", { name: "Sign in" })).toBeEnabled();
    await expectUnauthenticated(context);

    tracker.assertClean();
  });

  test("signs in through the UI and honors safe redirect handling", async ({
    page,
    context,
  }) => {
    const env = readE2EEnv();
    let tracker = trackLoginBrowserIssues(page);

    await logoutViaApi(context.request);
    await page.goto("/login", {
      waitUntil: "networkidle",
    });
    await expectLoginControls(page);

    const loginResponse = await submitLogin(page, env.email, env.password);
    expect(loginResponse.status()).toBe(200);

    const loginBody = await readJSON<unknown>(loginResponse);
    expect(loginBody.error, loginBody.message).toBe(false);

    await expect(page).toHaveURL(appPath("/"));
    await page.waitForLoadState("networkidle");
    await expectAuthenticated(context);
    tracker.assertClean();

    await logoutViaApi(context.request);
    await page.goto("/login?redirect=/settings/account", {
      waitUntil: "networkidle",
    });
    tracker = trackLoginBrowserIssues(page);
    await expectLoginControls(page);

    const safeRedirectLoginResponse = await submitLogin(
      page,
      env.email,
      env.password,
    );
    expect(safeRedirectLoginResponse.status()).toBe(200);

    await expect(page).toHaveURL(appPath("/settings/account"));
    await expectAuthenticated(context);
    tracker.assertClean();

    await page.goto("/login?redirect=/settings/account", {
      waitUntil: "networkidle",
    });
    await expect(page).toHaveURL(appPath("/settings/account"));

    await logoutViaApi(context.request);
    await page.goto("/login?redirect=https://example.com", {
      waitUntil: "networkidle",
    });
    tracker = trackLoginBrowserIssues(page);
    await expectLoginControls(page);

    const unsafeRedirectLoginResponse = await submitLogin(
      page,
      env.email,
      env.password,
    );
    expect(unsafeRedirectLoginResponse.status()).toBe(200);
    await expect(page).toHaveURL(appPath("/"));
    await page.waitForLoadState("networkidle");

    expect(new URL(page.url()).origin).toBe(new URL(env.baseURL).origin);
    await expectAuthenticated(context);

    tracker.assertClean();
  });

  test("logs out through the sidebar and blocks protected routes", async ({
    page,
    context,
  }) => {
    const env = readE2EEnv();
    const tracker = trackLoginBrowserIssues(page);

    await logoutViaApi(context.request);
    await page.goto("/login?redirect=/settings/account", {
      waitUntil: "networkidle",
    });
    await expectLoginControls(page);

    const loginResponse = await submitLogin(page, env.email, env.password);
    expect(loginResponse.status()).toBe(200);

    await expect(page).toHaveURL(appPath("/settings/account"));
    await expectAuthenticated(context);

    const logoutResponsePromise = page.waitForResponse(
      response =>
        response.url().includes("/api/auth/logout") &&
        response.request().method() === "DELETE",
    );

    await page.getByRole("button", { name: "Logout" }).click();

    const logoutResponse = await logoutResponsePromise;
    expect(logoutResponse.status()).toBe(200);

    const logoutBody = await readJSON<unknown>(logoutResponse);
    expect(logoutBody.error, logoutBody.message).toBe(false);

    await expect(page).toHaveURL(appPath("/login"));
    await expectUnauthenticated(context);

    await page.goto("/settings/account", {
      waitUntil: "networkidle",
    });
    await expect(page).toHaveURL(appPath("/login"));
    await expect
      .poll(() => new URL(page.url()).searchParams.get("redirect"))
      .toBe("/settings/account");

    tracker.assertClean();
  });

  test("is responsive and keeps screen reader support at each viewport", async ({
    page,
    context,
  }) => {
    const tracker = trackLoginBrowserIssues(page);

    await logoutViaApi(context.request);

    for (const viewport of Object.values(VIEWPORTS)) {
      await page.setViewportSize(viewport);
      await page.goto("/login", { waitUntil: "networkidle" });
      await expectLoginControls(page);
      await expectLoginLayout(page);
    }

    tracker.assertClean();
  });
});
