import {
  expect,
  type APIRequestContext,
  type Page,
  type Route,
} from "@playwright/test";
import {
  movieScanStatus,
  musicScanStatus,
  showScanStatus,
} from "../src/test/helpers/scan-status";

export {
  nullableFloat64,
  nullableInt64,
  nullableString,
} from "../src/test/helpers/fixtures";

// Shared shapes for talking to the API from specs: the JSON envelope, and the
// two ways a spec produces a body — reading a real response, or fulfilling an
// intercepted route.

type ApiResponse<T> = {
  error: boolean;
  message?: string;
  data?: T;
};

/**
 * Reads the JSON envelope off anything response-shaped — Playwright's
 * `APIResponse` and `Response`, or a fetch `Response`. Typed structurally so
 * callers do not have to agree on which `Response` they mean.
 */
export async function readJSON<T>(response: { json(): Promise<unknown> }) {
  return (await response.json()) as ApiResponse<T>;
}

/**
 * GETs `path` on a request context that already holds a session, asserts a
 * 200 success envelope carrying data, and returns the data.
 */
export async function getApiData<T>(request: APIRequestContext, path: string) {
  const response = await request.get(path, { failOnStatusCode: false });
  expect(response.status(), `GET ${path}`).toBe(200);

  const body = await readJSON<T>(response);
  expect(body.error, body.message).toBe(false);
  expect(body.data).toBeTruthy();
  return body.data!;
}

/** Wraps data in the success envelope. */
export function apiResponse<T>(data: T): ApiResponse<T> {
  return {
    error: false,
    data,
  };
}

export async function fulfillJSON(route: Route, body: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

/**
 * A page-based list envelope (movies, shows) echoing the request's `page`,
 * `per_page` and `sort`, with `items` under `key`.
 */
export function pagedList<K extends string, T>(
  url: URL,
  key: K,
  items: T[],
  { total, perPage }: { total: number; perPage: number },
) {
  const requestedPerPage = Number(url.searchParams.get("per_page") ?? perPage);

  return {
    [key]: items,
    total,
    page: Number(url.searchParams.get("page") ?? "1"),
    per_page: requestedPerPage,
    total_pages: Math.max(1, Math.ceil(total / requestedPerPage)),
    sort: url.searchParams.get("sort") === "desc" ? "desc" : "asc",
  } as Record<K, T[]> & {
    total: number;
    page: number;
    per_page: number;
    total_pages: number;
    sort: "asc" | "desc";
  };
}

/**
 * Waits until one of the recorded requests hit `pathname` with every entry of
 * `params` in its query. Specs push each `URL` a mock handler sees, so the
 * poll covers requests that land after the UI has already moved on.
 */
export async function expectApiRequest(
  requests: URL[],
  pathname: string,
  params: Record<string, string>,
) {
  await expect
    .poll(
      () =>
        requests.some(
          url =>
            url.pathname === pathname &&
            Object.entries(params).every(([key, value]) => url.searchParams.get(key) === value),
        ),
      { message: `expected a request for ${pathname} with ${JSON.stringify(params)}` },
    )
    .toBe(true);
}

/**
 * Holds every request matching `pattern` until `release()`, then answers it
 * with `respond` — by default whatever the next route handler, or the server,
 * would have answered.
 */
export async function gateRoute(
  page: Page,
  pattern: string | RegExp,
  respond: (route: Route) => Promise<void> = route => route.fallback(),
) {
  let release!: () => void;
  const gate = new Promise<void>(resolve => {
    release = resolve;
  });
  let requested = false;

  await page.route(pattern, async route => {
    requested = true;
    await gate;
    await respond(route);
  });

  return { release, requested: () => requested };
}

// The authenticated app shell polls all three scan-status endpoints on every
// route for admins (it discovers a scan already in flight), so every spec that
// serves an admin session sees them regardless of the page under test.

/** Overrides that turn a scan-status builder's report into an idle one. */
export const IDLE_SCAN = {
  run_id: "",
  state: "idle",
  phase: "idle",
  total: 0,
  started_at: null,
  updated_at: null,
} as const;

export const IDLE_SCAN_STATUS_BY_PATH: Record<string, () => unknown> = {
  "/api/settings/scan/movies": () => movieScanStatus(IDLE_SCAN),
  "/api/settings/scan/music": () => musicScanStatus(IDLE_SCAN),
  "/api/settings/scan/shows": () => showScanStatus(IDLE_SCAN),
};

/**
 * Fulfills the app shell's scan-status polling with an idle report.
 * Returns true when it handled the route, so callers can `return` immediately.
 */
export async function fulfillIdleScanStatus(route: Route, pathname: string) {
  const status = IDLE_SCAN_STATUS_BY_PATH[pathname];
  if (!status) {
    return false;
  }

  await fulfillJSON(route, apiResponse(status()));
  return true;
}
