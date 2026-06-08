import { useEffect, useRef, useState } from "react";

type RunContentFadeTransitionArgs = {
  onTransition: () => void | Promise<void>;
  shouldAnimate?: boolean;
};

function getPrefersReducedMotion() {
  if (typeof window === "undefined") {
    return false;
  }

  return window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false;
}

export function useContentFadeTransition(
  transitionMs: number,
) {
  const exitTimerRef = useRef<number | null>(null);
  const [isExiting, setIsExiting] = useState(false);
  const [prefersReducedMotion, setPrefersReducedMotion] = useState(
    getPrefersReducedMotion,
  );

  useEffect(() => {
    if (typeof window === "undefined" || !window.matchMedia) {
      return;
    }

    const mediaQuery = window.matchMedia("(prefers-reduced-motion: reduce)");
    const syncReducedMotion = () => {
      setPrefersReducedMotion(mediaQuery.matches);
    };

    syncReducedMotion();
    mediaQuery.addEventListener("change", syncReducedMotion);

    return () => {
      mediaQuery.removeEventListener("change", syncReducedMotion);
    };
  }, []);

  useEffect(() => {
    return () => {
      if (exitTimerRef.current !== null) {
        window.clearTimeout(exitTimerRef.current);
      }
    };
  }, []);

  const clearPendingTransition = () => {
    if (exitTimerRef.current !== null) {
      window.clearTimeout(exitTimerRef.current);
      exitTimerRef.current = null;
    }
    setIsExiting(false);
  };

  const runTransition = ({
    onTransition,
    shouldAnimate = true,
  }: RunContentFadeTransitionArgs) => {
    clearPendingTransition();

    if (!shouldAnimate || prefersReducedMotion) {
      void onTransition();
      return;
    }

    setIsExiting(true);
    exitTimerRef.current = window.setTimeout(() => {
      exitTimerRef.current = null;
      setIsExiting(false);
      void onTransition();
    }, transitionMs);
  };

  return {
    isExiting,
    usesContentAnimation: true,
    runTransition,
  };
}
