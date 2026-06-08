import type { Request } from "@playwright/test";

export function isIgnorableFailedRequest(request: Request) {
  const failureText = request.failure()?.errorText ?? "";
  if (!failureText.includes("net::ERR_ABORTED")) {
    return false;
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
