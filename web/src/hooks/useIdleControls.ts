import { useEffect, useRef, useState } from "react";

type IdleControlsOptions = {
  active: boolean;
  idleMs: number;
};

export function useIdleControls({ active, idleMs }: IdleControlsOptions) {
  const [visible, setVisible] = useState(true);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const clearTimer = () => {
    if (timerRef.current) {
      clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  };

  const showAndReset = () => {
    clearTimer();
    setVisible(true);
    if (active) {
      timerRef.current = setTimeout(() => {
        setVisible(false);
        timerRef.current = null;
      }, idleMs);
    }
  };

  useEffect(() => {
    if (!active) {
      clearTimer();
      // Reset visibility for next entry; deferred to avoid synchronous setState in effect.
      queueMicrotask(() => setVisible(true));
      return;
    }
    queueMicrotask(() => setVisible(true));
    timerRef.current = setTimeout(() => {
      setVisible(false);
      timerRef.current = null;
    }, idleMs);
    return clearTimer;
  }, [active, idleMs]);

  return { visible, showAndReset };
}
