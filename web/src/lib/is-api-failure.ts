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
 */
export function apiErrorMessage(data: unknown, fallback: string): string {
  if (!isApiFailure(data)) return fallback;
  const status = (data as { status?: unknown }).status;
  if (typeof status === "number" && (status === 404 || status >= 500)) {
    return fallback;
  }
  return data.message || fallback;
}
