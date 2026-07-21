import { useRef, type RefObject } from "react";

type Options = {
  fallbackRef?: RefObject<HTMLElement | null>;
};

function getConnectedElement(element: HTMLElement | null | undefined) {
  return element?.isConnected ? element : null;
}

export function focusDialogRestoreTarget(
  target: HTMLElement | null | undefined,
  fallback?: HTMLElement | null,
) {
  const focusTarget = getConnectedElement(target) ?? getConnectedElement(fallback);
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
