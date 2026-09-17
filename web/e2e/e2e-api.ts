import type { Route } from "@playwright/test";
import { movieScanStatus } from "../src/test/helpers/movie-scan";
import { musicScanStatus } from "../src/test/helpers/music-scan";
import { showScanStatus } from "../src/test/helpers/show-scan";

// Shared shapes for talking to the API from specs: the JSON envelope, the Go
// `sql.Null*` wire types, and the two ways a spec produces a body — reading a
// real response, or fulfilling an intercepted route.

export type ApiResponse<T> = {
  error: boolean;
  message?: string;
  data?: T;
};

export type NullableString = {
  String: string;
  Valid: boolean;
};

export type NullableInt64 = {
  Int64: number;
  Valid: boolean;
};

type NullableFloat64 = {
  Float64: number;
  Valid: boolean;
};

/**
 * Reads the JSON envelope off anything response-shaped — Playwright's
 * `APIResponse` and `Response`, or a fetch `Response`. Typed structurally so
 * callers do not have to agree on which `Response` they mean.
 */
export async function readJSON<T>(response: { json(): Promise<unknown> }) {
  return (await response.json()) as ApiResponse<T>;
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

export function nullableString(value = ""): NullableString {
  return {
    String: value,
    Valid: value.length > 0,
  };
}

export function nullableInt64(value: number | null = null): NullableInt64 {
  return {
    Int64: value ?? 0,
    Valid: value != null,
  };
}

export function nullableFloat64(value: number | null = null): NullableFloat64 {
  return {
    Float64: value ?? 0,
    Valid: value != null,
  };
}

// The authenticated app shell polls all three scan-status endpoints on every
// route for admins (it discovers a scan already in flight), so every spec that
// owns a catch-all `**/api/**` mock sees them regardless of the page under test.
// Specs call this first in their handler and return when it reports handled,
// so the requests do not land in their unexpected-request assertions.
const IDLE_SCAN = {
  run_id: "",
  state: "idle",
  phase: "idle",
  total: 0,
  started_at: null,
  updated_at: null,
} as const;

const IDLE_SCAN_STATUS_BY_PATH: Record<string, () => unknown> = {
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
