// Fetch-mocking primitives shared by the route and dialog suites: build a JSON
// `Response`, normalize whatever `RequestInfo` form the caller passed, count
// how often a URL was requested, and (`stubApi`) answer a whole page's
// requests from a route table.

import { expect, onTestFinished, vi } from "vitest";
import type { AuthUser } from "@/types";
import { authUser } from "./fixtures";

/** Resolves to a JSON `Response`, matching what the real API returns. */
export function jsonResponse(body: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

/** Normalizes the three `fetch` input forms to a plain URL string. */
export function requestURL(input: RequestInfo | URL) {
  if (typeof input === "string") return input;
  if (input instanceof URL) return input.toString();
  return input.url;
}

/** How many times a stubbed `fetch` was called with exactly `url`. */
export function countFetchRequests(
  fetchMock: { mock: { calls: [RequestInfo | URL, ...unknown[]][] } },
  url: string,
) {
  return fetchMock.mock.calls.filter(([input]) => requestURL(input) === url)
    .length;
}

/**
 * A `fetch` result the test resolves by hand, for asserting what a component
 * shows while a request is still in flight (an optimistic update, a pending
 * state) before deciding how the server answers.
 */
export function deferredResponse() {
  let resolve!: (response: Response | PromiseLike<Response>) => void;
  const promise = new Promise<Response>(resolvePromise => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}

/** A request a `stubApi` route answers. */
export type StubApiRequest = {
  /** The URL as fetched, query included. */
  url: string;
  /** The URL's pathname, without the query. */
  path: string;
  search: URLSearchParams;
  method: string;
  init?: RequestInit;
};

/**
 * What a route answers: a `Response` is returned as is, so a test can send an
 * error status; anything else is sent as the `{ error: false, data }`
 * envelope. A route may return a promise of either, to hold a request open.
 */
export type StubApiRoute =
  | ((request: StubApiRequest) => unknown)
  | object
  | string
  | number
  | boolean
  | null;

type StubApiOptions = {
  /**
   * The `GET /api/auth/user` user, from `authUserData`; `null` leaves the
   * route to the table. Defaults to the standard test user.
   */
  user?: Partial<AuthUser> | null;
};

/**
 * Stubs `fetch` with a route table and returns the mock.
 *
 * Keys are `"METHOD /path"` or `"/path"` (any method), matched first with
 * the query (`"/api/x?page=2"`) and then without it. A request no route
 * answers gets a 404 envelope and fails the test when it finishes, so a
 * stray call cannot pass silently.
 *
 *   stubApi({
 *     "/api/movies/latest": { movies: [movie()] },
 *     "POST /api/movies/7/like": () => jsonResponse({ error: true }, 500),
 *   });
 */
export function stubApi(
  routes: Record<string, StubApiRoute>,
  { user = {} }: StubApiOptions = {},
) {
  const table: Record<string, StubApiRoute> = { ...routes };
  if (user !== null && !("/api/auth/user" in table) && !("GET /api/auth/user" in table)) {
    table["GET /api/auth/user"] = () => jsonResponse(authUser(user));
  }

  const unhandled: string[] = [];
  onTestFinished(() => {
    expect(unhandled, "requests no stubApi route answered").toEqual([]);
  });

  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = requestURL(input);
    const parsed = new URL(url, "http://localhost");
    const method = (
      init?.method ?? (input instanceof Request ? input.method : "GET")
    ).toUpperCase();
    const key = [
      `${method} ${url}`,
      url,
      `${method} ${parsed.pathname}`,
      parsed.pathname,
    ].find(candidate => candidate in table);

    if (key === undefined) {
      unhandled.push(`${method} ${url}`);
      return jsonResponse({ error: true, message: "Unhandled stubApi request" }, 404);
    }

    const route = table[key];
    // An async route may resolve to a Response or to plain data alike.
    const answer: unknown = await (typeof route === "function"
      ? route({ url, path: parsed.pathname, search: parsed.searchParams, method, init })
      : route);
    if (answer instanceof Response) return answer;
    return jsonResponse({ error: false, data: answer });
  });

  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}
