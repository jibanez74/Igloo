import { useLayoutEffect, useRef, type RefObject } from "react";

type Options = {
  /**
   * Give focus back to the button once it is enabled again, if focus is
   * still where the handoff left it. For a target that is not a control: a
   * reader cannot have started using it meanwhile.
   */
  returns?: boolean;
};

/**
 * Keeps keyboard focus when a button disables itself. A button that saves
 * and then has nothing left to do is disabled while it still holds focus; a
 * disabled element cannot keep it, so the browser moves focus to the page
 * body and the next Tab starts from the top. This hands focus to `targetRef`
 * instead, for any button inside `scopeRef`.
 *
 * It runs in a layout effect on every render: the DOM already has the
 * `disabled` attribute, and the browser has not yet taken focus away.
 */
export function useFocusHandoff(
  scopeRef: RefObject<HTMLElement | null>,
  targetRef: RefObject<HTMLElement | null>,
  { returns = false }: Options = {},
) {
  const handedOffFromRef = useRef<HTMLButtonElement | null>(null);

  useLayoutEffect(() => {
    const scope = scopeRef.current;
    const target = targetRef.current;
    if (!scope || !target) return;

    const active = document.activeElement;

    if (
      active instanceof HTMLButtonElement &&
      active.disabled &&
      scope.contains(active)
    ) {
      handedOffFromRef.current = active;
      target.focus({ preventScroll: true });
      return;
    }

    const from = handedOffFromRef.current;
    if (!from) return;

    // Focus has moved on by itself: there is nothing left to give back.
    if (active !== target) {
      handedOffFromRef.current = null;
      return;
    }

    if (returns && from.isConnected && !from.disabled) {
      handedOffFromRef.current = null;
      from.focus({ preventScroll: true });
    }
  });
}
