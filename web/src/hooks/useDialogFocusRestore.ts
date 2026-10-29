import { useRef, type RefObject } from "react";

type Options = {
  fallbackRef?: RefObject<HTMLElement | null>;
};

// A detached or disabled element cannot take focus, so the fallback does.
function getFocusableElement(element: HTMLElement | null | undefined) {
  if (!element?.isConnected || element.matches(":disabled")) return null;
  return element;
}

export function focusDialogRestoreTarget(
  target: HTMLElement | null | undefined,
  fallback?: HTMLElement | null,
) {
  const focusTarget = getFocusableElement(target) ?? getFocusableElement(fallback);
  if (!focusTarget) return;

  const focus = () => focusTarget.focus({ preventScroll: true });

  if (typeof window === "undefined") {
    focus();
    return;
  }

  const schedule: (callback: FrameRequestCallback) => void =
    typeof window.requestAnimationFrame === "function"
      ? callback => window.requestAnimationFrame(callback)
      : callback => window.setTimeout(callback, 0);
  schedule(focus);
}

export function useDialogFocusRestore(options: Options = {}) {
  const restoreFocusRef = useRef<HTMLElement | null>(null);

  const setRestoreFocusTarget = (target: HTMLElement | null) => {
    restoreFocusRef.current = target;
  };

  const restoreFocus = () => {
    focusDialogRestoreTarget(
      restoreFocusRef.current,
      options.fallbackRef?.current,
    );
  };

  const onCloseAutoFocus = (event: Event) => {
    event.preventDefault();
    restoreFocus();
  };

  return {
    restoreFocusRef,
    setRestoreFocusTarget,
    restoreFocus,
    onCloseAutoFocus,
  };
}
