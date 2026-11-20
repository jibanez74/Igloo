import type { ConsoleMessage, Page, Request, Response } from "@playwright/test";

function isIgnorableFailedRequest(request: Request) {
  const failureText = request.failure()?.errorText ?? "";
  if (!failureText.includes("net::ERR_ABORTED")) {
    return false;
  }

  // React Query aborts a query's GET once nothing observes it any more: a
  // page left mid-load, a superseded search, or StrictMode's first mount in
  // the dev server.
  const url = new URL(request.url());
  if (
    request.method() === "GET" &&
    url.pathname.startsWith("/api/") &&
    request.resourceType() === "fetch"
  ) {
    return true;
  }

  // hls.js loads playlists and segments over XHR/fetch and aborts them when
  // the player is torn down or re-sourced, e.g. a seek that restarts the
  // session.
  if (
    ["fetch", "xhr"].includes(request.resourceType()) &&
    url.pathname.includes("/hls/")
  ) {
    return true;
  }

  // A media fetch is aborted whenever its element is torn down or reloaded,
  // e.g. the episode hand-off unmounting a player whose stream was pending.
  return ["font", "image", "media", "script", "stylesheet"].includes(
    request.resourceType(),
  );
}

export function isExpectedUnauthorizedResourceMessage(message: string) {
  return (
    message ===
    "Failed to load resource: the server responded with a status of 401 (Unauthorized)"
  );
}

/**
 * The browser's "Failed to load resource" echo of an `/api/` response. The
 * response itself is judged by `ignoreResponse`, so a spec can drop its echo;
 * an echo for anything else (a chunk, a font, an icon) still counts.
 */
export function isApiResourceEcho(text: string, message: ConsoleMessage) {
  return (
    text.startsWith("Failed to load resource:") &&
    new URL(message.location().url, "http://localhost").pathname.startsWith("/api/")
  );
}

function isSignedOutAuthCheck(response: Response) {
  return (
    response.status() === 401 &&
    response.request().method() === "GET" &&
    new URL(response.url()).pathname === "/api/auth/user"
  );
}

/**
 * A signed-out page asks who is signed in and hears 401. That answer, and
 * the browser's console echo of it, are the only errors such a page expects.
 */
export const signedOutIssueRules: TrackBrowserIssuesOptions = {
  ignoreResponse: isSignedOutAuthCheck,
  ignoreConsole: (_type, text, message) =>
    isExpectedUnauthorizedResourceMessage(text) &&
    new URL(message.location().url, "http://localhost").pathname === "/api/auth/user",
};

/** Once the session ends, whatever the open page asks for is refused. */
export function isSessionEndedResponse(response: Response) {
  return (
    response.status() === 401 &&
    new URL(response.url()).pathname.startsWith("/api/")
  );
}

/** A refusal a test asks the server (or its stub) for. */
export type ExpectedRefusal = { method: string; path: string; status: number };

/**
 * Drops the refusals a test drives on purpose, and the browser's console
 * echo of them, and nothing else: another status, method or route still
 * fails. It does not count them; a test that cares how many were sent
 * asserts that itself.
 */
export function expectedRefusalRules(
  ...refusals: ExpectedRefusal[]
): TrackBrowserIssuesOptions {
  return {
    ignoreResponse: response =>
      refusals.some(
        refusal =>
          response.status() === refusal.status &&
          response.request().method() === refusal.method &&
          new URL(response.url()).pathname === refusal.path,
      ),
    ignoreConsole: (_type, text, message) =>
      refusals.some(
        refusal =>
          text.startsWith(`Failed to load resource: the server responded with a status of ${refusal.status} `) &&
          new URL(message.location().url, "http://localhost").pathname === refusal.path,
      ),
  };
}

function isAppApiResponse(response: Response) {
  return new URL(response.url()).pathname.startsWith("/api/");
}

export type TrackBrowserIssuesOptions = {
  /**
   * Console errors/warnings this spec expects. Returning true drops the
   * message instead of failing the test.
   */
  ignoreConsole?: (type: string, text: string, message: ConsoleMessage) => boolean;
  /**
   * `/api/` error responses this spec drives on purpose, such as a 409 for a
   * duplicate email. Returning true drops the response instead of failing.
   */
  ignoreResponse?: (response: Response) => boolean;
  /**
   * Failed requests this spec causes on purpose, such as an outage it
   * simulates. Returning true drops the request instead of failing.
   */
  ignoreRequestFailure?: (request: Request) => boolean;
  /** Set false for specs that drive flows React warns about. Defaults to true. */
  trackConsoleWarnings?: boolean;
};

export type BrowserIssueTracker = ReturnType<typeof trackBrowserIssues>;

/**
 * Watches one page. Specs do not call this: the shared `test` in e2e-test.ts
 * tracks every page and asserts at teardown.
 */
export function trackBrowserIssues(
  page: Page,
  options: TrackBrowserIssuesOptions = {},
) {
  const ignoreConsoleRules = options.ignoreConsole ? [options.ignoreConsole] : [];
  const ignoreResponseRules = options.ignoreResponse ? [options.ignoreResponse] : [];
  const ignoreFailureRules = options.ignoreRequestFailure ? [options.ignoreRequestFailure] : [];
  let trackConsoleWarnings = options.trackConsoleWarnings ?? true;
  const consoleIssues: string[] = [];
  const pageErrors: string[] = [];
  const failedRequests: string[] = [];
  const responseErrors: string[] = [];

  page.on("console", message => {
    const isTracked =
      message.type() === "error" ||
      (trackConsoleWarnings && message.type() === "warning");
    if (!isTracked) {
      return;
    }

    if (ignoreConsoleRules.some(rule => rule(message.type(), message.text(), message))) {
      return;
    }

    consoleIssues.push(`${message.type()}: ${message.text()}`);
  });
  page.on("pageerror", error => pageErrors.push(error.message));
  page.on("requestfailed", request => {
    if (isIgnorableFailedRequest(request) || ignoreFailureRules.some(rule => rule(request))) {
      return;
    }

    failedRequests.push(
      `${request.method()} ${request.url()} ${request.failure()?.errorText ?? ""}`,
    );
  });
  page.on("response", response => {
    if (
      isAppApiResponse(response) &&
      response.status() >= 400 &&
      !ignoreResponseRules.some(rule => rule(response))
    ) {
      responseErrors.push(
        `${response.status()} ${response.request().method()} ${response.url()}`,
      );
    }
  });

  return {
    /** Adds rules to the ones the tracker started with. */
    ignore(more: TrackBrowserIssuesOptions) {
      if (more.ignoreConsole) ignoreConsoleRules.push(more.ignoreConsole);
      if (more.ignoreResponse) ignoreResponseRules.push(more.ignoreResponse);
      if (more.ignoreRequestFailure) ignoreFailureRules.push(more.ignoreRequestFailure);
      if (more.trackConsoleWarnings !== undefined) {
        trackConsoleWarnings = more.trackConsoleWarnings;
      }
    },
    issues() {
      return { consoleIssues, pageErrors, failedRequests, responseErrors };
    },
  };
}
