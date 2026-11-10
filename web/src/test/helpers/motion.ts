// Locators for the three-stage entrance stagger the detail pages share
// (movie, album, musician). The wrappers are plain divs carrying the animation
// utilities, so there is no role or label to query them by.

/**
 * The utilities every staggered detail-page wrapper carries. Matched as a
 * set: the order of classes in a string is the linter's, not a contract.
 */
const DETAIL_PAGE_ANIMATION_MARKERS = [
  "animate-in",
  "fade-in",
  "slide-in-from-bottom-2",
];

export function getDetailMotionWrappers(container: HTMLElement) {
  return Array.from(container.querySelectorAll("div")).filter(element =>
    DETAIL_PAGE_ANIMATION_MARKERS.every(marker =>
      element.classList.contains(marker),
    ),
  );
}

/** The second stage of the stagger. */
export function getHeroMotionWrapper(container: HTMLElement) {
  return getDetailMotionWrappers(container).find(element =>
    element.className.includes("delay-75"),
  );
}

/** The third stage of the stagger. */
export function getLowerMotionWrapper(container: HTMLElement) {
  return getDetailMotionWrappers(container).find(element =>
    element.className.includes("delay-150"),
  );
}
