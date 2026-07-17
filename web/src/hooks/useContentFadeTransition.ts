import { useEffect, useState } from "react";
import { getPrefersReducedMotion } from "@/lib/motion";

type RunContentFadeTransitionArgs = {
  onTransition: () => void | Promise<void>;
  shouldAnimate?: boolean;
};

type PendingContentFadeTransition = {
  onTransition: () => void | Promise<void>;
};

export function useContentFadeTransition(
  transitionMs: number,
) {
  const [isExiting, setIsExiting] = useState(false);
  const [pendingTransition, setPendingTransition] =
    useState<PendingContentFadeTransition | null>(null);
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
    if (!pendingTransition) return;

    let isCurrentTransition = true;
    const timeoutId = window.setTimeout(() => {
      if (!isCurrentTransition) return;

      setIsExiting(false);
      setPendingTransition(currentTransition => (
        currentTransition === pendingTransition ? null : currentTransition
      ));
      void pendingTransition.onTransition();
    }, transitionMs);

    return () => {
      isCurrentTransition = false;
      window.clearTimeout(timeoutId);
    };
  }, [pendingTransition, transitionMs]);

  const clearPendingTransition = () => {
    setPendingTransition(null);
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
    setPendingTransition({ onTransition });
  };

  return {
    isExiting,
    usesContentAnimation: true,
    runTransition,
  };
}
