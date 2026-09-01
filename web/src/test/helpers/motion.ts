// Locators for the three-stage entrance stagger the detail pages share
// (movie, album, musician). The wrappers are plain divs carrying the animation
// utilities, so there is no role or label to query them by.

/** The utility set every staggered detail-page wrapper carries. */
const DETAIL_PAGE_ANIMATION_MARKER =
  "animate-in fade-in slide-in-from-bottom-2";

export function getDetailMotionWrappers(container: HTMLElement) {
  return Array.from(container.querySelectorAll("div")).filter(element =>
    element.className.includes(DETAIL_PAGE_ANIMATION_MARKER),
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
