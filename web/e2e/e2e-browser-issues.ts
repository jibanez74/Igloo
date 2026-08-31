import { expect, type Page, type Request, type Response } from "@playwright/test";

export function isIgnorableFailedRequest(request: Request) {
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

  return ["font", "image", "script", "stylesheet"].includes(
    request.resourceType(),
  );
}

export function isExpectedUnauthorizedResourceMessage(message: string) {
  return (
    message ===
    "Failed to load resource: the server responded with a status of 401 (Unauthorized)"
  );
}

export function isAppApiResponse(response: Response) {
  return new URL(response.url()).pathname.startsWith("/api/");
}

export type TrackBrowserIssuesOptions = {
  /**
   * Console errors/warnings this spec expects. Returning true drops the
   * message instead of failing `assertClean`.
   */
  ignoreConsole?: (type: string, text: string) => boolean;
  /**
   * Lowest `/api/` response status counted as an error. Defaults to 400;
   * specs that deliberately drive 4xx flows raise it to 500.
   */
  minResponseStatus?: number;
  /** Set false for specs that drive flows React warns about. Defaults to true. */
  trackConsoleWarnings?: boolean;
};

export function trackBrowserIssues(
  page: Page,
  options: TrackBrowserIssuesOptions = {},
) {
  const {
    ignoreConsole,
    minResponseStatus = 400,
    trackConsoleWarnings = true,
  } = options;
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
    if (isAppApiResponse(response) && response.status() >= minResponseStatus) {
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
