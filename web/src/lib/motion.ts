/** True when the user prefers reduced motion; false in non-DOM environments. */
export function getPrefersReducedMotion(): boolean {
  if (typeof window === "undefined") {
    return false;
  }

  return window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false;
}
