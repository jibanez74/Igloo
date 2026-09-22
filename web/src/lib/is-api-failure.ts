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
 * The server's own message when the envelope reports a failure, and the
 * caller's wording when the request never got that far — a network error, or a
 * response that was not an envelope at all. Every load-error surface makes the
 * same choice, so it is made here.
 */
export function apiErrorMessage(data: unknown, fallback: string): string {
  return isApiFailure(data) ? data.message : fallback;
}
