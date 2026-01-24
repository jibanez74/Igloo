/**
 * Utilities for safe redirect handling to prevent open redirect vulnerabilities.
 *
 * Open redirect attacks occur when an attacker crafts a URL with a malicious
 * redirect parameter that sends users to an external site after login.
 *
 * Examples of dangerous redirects:
 * - //evil.com (protocol-relative URL)
 * - https://evil.com (absolute external URL)
 * - javascript:alert(1) (javascript protocol)
 */

/**
 * Validates that a redirect URL is safe (internal, relative path only).
 *
 * @param url - The redirect URL to validate
 * @returns true if the URL is safe to redirect to
 *
 * @example
 * isValidRedirect("/") // true
 * isValidRedirect("/music") // true
 * isValidRedirect("/music/album/1") // true
 * isValidRedirect("//evil.com") // false
 * isValidRedirect("https://evil.com") // false
 * isValidRedirect("javascript:alert(1)") // false
 */
export function isValidRedirect(url: string): boolean {
  // Must start with a single forward slash (relative path)
  if (!url.startsWith("/")) {
    return false;
  }

  // Must not start with // (protocol-relative URL that could redirect externally)
  if (url.startsWith("//")) {
    return false;
  }

  // Must not contain protocol indicators that could be used for attacks
  // This catches edge cases like "/\evil.com" or "/ /evil.com"
  if (url.includes("://") || url.includes("\\")) {
    return false;
  }

  return true;
}

/**
 * Returns a safe redirect URL, falling back to "/" if the input is invalid.
 *
 * @param url - The redirect URL to sanitize
 * @param fallbackUrl - The URL to return if input is invalid (default: "/")
 * @returns A safe redirect URL
 */
export function getSafeRedirect(url: string, fallbackUrl: string = "/"): string {
  return isValidRedirect(url) ? url : fallbackUrl;
}
