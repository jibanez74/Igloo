export const REDUCED_MOTION_QUERY = "(prefers-reduced-motion: reduce)";

/** True when the user prefers reduced motion; false in non-DOM environments. */
export function getPrefersReducedMotion(): boolean {
  if (typeof window === "undefined") {
    return false;
  }

  return window.matchMedia?.(REDUCED_MOTION_QUERY).matches ?? false;
}

/** Scrolls the window to the top, instant when the user prefers reduced motion. */
export function scrollWindowToTop(): void {
  window.scrollTo({
    top: 0,
    behavior: getPrefersReducedMotion() ? "auto" : "smooth",
  });
}
