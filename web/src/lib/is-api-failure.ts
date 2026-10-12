import type { ApiFailureType, ApiResponseType } from "@/types";

/**
 * What a query throws when it must fail as a query instead of caching the
 * failure envelope (the infinite track lists, whose pages cannot hold one).
 * It carries the envelope, so apiErrorMessage words it like any other failure.
 */
export class ApiFailureError extends Error {
  readonly failure: ApiFailureType;

  constructor(failure: ApiFailureType) {
    super(failure.message);
    this.name = "ApiFailureError";
    this.failure = failure;
  }
}

/** A successful envelope's payload; a failure envelope throws ApiFailureError. */
export function unwrapApiData<T extends Record<string, unknown>>(
  res: ApiResponseType<T>,
): T {
  if (res.error) throw new ApiFailureError(res);
  return res.data;
}

/** Type guard for API responses shaped as `{ error: true, message: string }`. */

export function isApiFailure(data: unknown): data is { error: true; message: string } {
  return (
    typeof data === "object" &&
    data !== null &&
    "error" in data &&
    (data).error === true &&
    "message" in data &&
    typeof (data as { message: string }).message === "string"
  );
}

/**
 * The server's own message when the envelope reports a client error the server
 * worded (a 4xx other than 404), and the caller's wording otherwise: a network
 * error, a response that was not an envelope at all, a blank message, a 404
 * (`apiRequest` swaps its body for a canned "404 - …" string), or a 5xx, whose
 * message is internal error text ("failed to add movies to playlist") rather
 * than copy written for a person. `apiRequest` stamps the status on every
 * failure. Every load-error surface makes the same choice, so it is made here.
 * It also takes an ApiFailureError, the envelope a throwing query rethrew.
 */
export function apiErrorMessage(error: unknown, fallback: string): string {
  const data = error instanceof ApiFailureError ? error.failure : error;
  if (!isApiFailure(data)) return fallback;
  const status = (data as { status?: unknown }).status;
  if (typeof status === "number" && (status === 404 || status >= 500)) {
    return fallback;
  }
  return data.message || fallback;
}
