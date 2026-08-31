import type { Route } from "@playwright/test";

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
