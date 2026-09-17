import { useState, useEffect } from "react";

export function usePrefersCoarsePointer() {
  const [coarse, setCoarse] = useState(() => {
    if (typeof window === "undefined") return false;
    return window.matchMedia("(hover: none) and (pointer: coarse)").matches;
  });

  useEffect(() => {
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
  