import { expect, type Page, type Request, type Response } from "@playwright/test";

function isIgnorableFailedRequest(request: Request) {
  const failureText = request.failure()?.errorText ?? "";
  if (!failureText.includes("net::ERR_ABORTED")) {
    return false;
  }

  const url = new URL(request.url());
  if (
    request.method() === "GET" &&
    url.pathname === "/api/auth/user" &&
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

function isAppApiResponse(response: Response) {
  return new URL(response.url()).pathname.startsWith("/api/");
}

type TrackBrowserIssuesOptions = {
  /**
   * Console errors/warnings this spec expects. Returning true drops the
   * message instead of failing `assertClean`.
   */
  ignoreConsole?: (type: string, text: string) => boolean;
  /**
   * `/api/` error responses this spec drives on purpose, such as a 409 for a
   * duplicate email. Returning true drops the response instead of failing.
   */
  ignoreResponse?: (response: Response) => boolean;
  /** Set false for specs that drive flows React warns about. Defaults to true. */
  trackConsoleWarnings?: boolean;
};

export function trackBrowserIssues(
  page: Page,
  options: TrackBrowserIssuesOptions = {},
) {
  const { ignoreConsole, ignoreResponse, trackConsoleWarnings = true } = options;
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

    if (ignoreConsole?.(message.type(), message.text())) {
      return;
    }

    consoleIssues.push(`${message.type()}: ${message.text()}`);
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
    if (
      isAppApiResponse(response) &&
      response.status() >= 400 &&
      !ignoreResponse?.(response)
    ) {
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

/**
 * For the specs that stub every `/api/**` call themselves: the run is clean
 * only if the page also asked for nothing the stub did not anticipate.
 */
export function assertMockSuiteClean(
  browserIssues: ReturnType<typeof trackBrowserIssues>,
  unexpectedApiRequests: string[],
) {
  expect(unexpectedApiRequests).toEqual([]);
  browserIssues.assertClean();
}
