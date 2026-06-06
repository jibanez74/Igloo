import { useEffect, useRef, useState } from "react";

type RunContentFadeTransitionArgs = {
  onTransition: () => void | Promise<void>;
  shouldAnimate?: boolean;
};

type UseContentFadeTransitionOptions = {
  enableViewTransition?: boolean;
};

export function useContentFadeTransition(
  transitionMs: number,
  { enableViewTransition = true }: UseContentFadeTransitionOptions = {},
) {
  const exitTimerRef = useRef<number | null>(null);
  const [isExiting, setIsExiting] = useState(false);
  const prefersReducedMotion =
    typeof window !== "undefined" &&
    (window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ?? false);
  const supportsViewTransition =
    enableViewTransition &&
    typeof document !== "undefined" &&
    "startViewTransition" in document;
  const usesViewTransition = supportsViewTransition && !prefersReducedMotion;
  const usesContentAnimation = !usesViewTransition;

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

    if (!shouldAnimate || supportsViewTransition || prefersReducedMotion) {
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
    usesContentAnimation,
    usesViewTransition,
    runTransition,
  };
}
