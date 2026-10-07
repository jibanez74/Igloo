import { SPLASH_REMOVE_DELAY_MS } from "@/lib/constants";

const SPLASH_ID = "initial-splash";

/**
 * Hands the page over from the pre-hydration splash to the rendered app.
 *
 * Marks the document ready (which `boot.css` turns into the splash's opacity
 * fade and `pointer-events: none`), hides the splash from assistive tech so
 * its `role="status"` is not announced beside `AppLoadingScreen`, and removes
 * the node once the fade ends. The removal is never cancelled: a watchdog
 * timer runs it even if `transitionend` never fires (reduced motion, a
 * backgrounded tab), and the whole function is idempotent so a remount can
 * call it again without re-arming anything. A splash that lingered used to
 * block every click, because its removal lived in an effect cleanup.
 */
export function dismissBootSplash() {
  const root = document.documentElement;
  const splash = document.getElementById(SPLASH_ID);

  root.setAttribute("data-app-ready", "true");

  if (!splash || splash.dataset.dismissed === "true") {
    return;
  }

  splash.dataset.dismissed = "true";
  splash.setAttribute("aria-hidden", "true");

  const remove = () => {
    window.clearTimeout(watchdog);
    splash.remove();
  };
  const watchdog = window.setTimeout(remove, SPLASH_REMOVE_DELAY_MS);

  splash.addEventListener("transitionend", remove, { once: true });
}
