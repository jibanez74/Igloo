import * as React from "react";

/**
 * True when the primary input is touch (e.g. phones, many tablets).
 * Prefer native form controls in these environments for reliable VoiceOver / TalkBack behavior.
 */
export function usePrefersCoarsePointer() {
  const [coarse, setCoarse] = React.useState(() => {
    if (typeof window === "undefined") return false;
    return window.matchMedia("(hover: none) and (pointer: coarse)").matches;
  });

  React.useEffect(() => {
    const mql = window.matchMedia("(hover: none) and (pointer: coarse)");
    const sync = () => {
      setCoarse(mql.matches);
    };
    sync();
    mql.addEventListener("change", sync);
    return () => {
      mql.removeEventListener("change", sync);
    };
  }, []);

  return coarse;
}
