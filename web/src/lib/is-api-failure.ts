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
