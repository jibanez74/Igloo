// The keys a native slider moves by; a focused slider keeps them.
const SLIDER_KEYS: ReadonlySet<string> = new Set([
  "ArrowLeft",
  "ArrowRight",
  "ArrowUp",
  "ArrowDown",
  "Home",
  "End",
  "PageUp",
  "PageDown",
]);

const TEXT_ENTRY_SELECTOR = 'textarea, select, input:not([type="range"])';
const SLIDER_SELECTOR = 'input[type="range"], [role="slider"]';
const ACTIVATABLE_SELECTOR = [
  "button",
  "a[href]",
  "summary",
  '[role="button"]',
  '[role="checkbox"]',
  '[role="menuitem"]',
  '[role="option"]',
  '[role="radio"]',
  '[role="switch"]',
  '[role="tab"]',
].join(", ");

/**
 * Whether a keydown belongs to the focused control rather than to a video
 * surface's playback shortcuts (Space/K, J/L, M, F, …):
 * - anything that takes typing owns every key;
 * - a slider (seek, volume) owns only the keys it moves by, so Space still
 *   plays and pauses after a drag left focus on it;
 * - a button, link or other activatable control owns Space and Enter;
 * - a key with Ctrl, Cmd or Alt is the browser's or the OS's, and a key a
 *   widget already handled (a menu, a dialog) is spent.
 */
export function shouldIgnorePlayerShortcut(event: KeyboardEvent): boolean {
  if (event.defaultPrevented || event.ctrlKey || event.metaKey || event.altKey) {
    return true;
  }

  const target = event.target;
  if (!(target instanceof HTMLElement)) return false;

  if (target.isContentEditable || target.closest(TEXT_ENTRY_SELECTOR)) {
    return true;
  }
  if (SLIDER_KEYS.has(event.key) && target.closest(SLIDER_SELECTOR)) {
    return true;
  }
  return (
    (event.key === " " || event.key === "Enter") &&
    target.closest(ACTIVATABLE_SELECTOR) !== null
  );
}
