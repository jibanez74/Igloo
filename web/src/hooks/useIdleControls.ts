import { useEffect, useRef, useState } from "react";

type IdleControlsOptions = {
  active: boolean;
  idleMs: number;
};

type IdleControlsState = {
  active: boolean;
  idleMs: number;
  visible: boolean;
};

type TimerRef = {
  current: ReturnType<typeof setTimeout> | null;
};

function clearIdleTimer(timerRef: TimerRef) {
  if (timerRef.current) {
    clearTimeout(timerRef.current);
    timerRef.current = null;
  }
}

function hideIdleControls(
  state: IdleControlsState,
  active: boolean,
  idleMs: number,
): IdleControlsState {
  if (!active || !state.active || state.idleMs !== idleMs || !state.visible) {
    return state;
  }

  return {
    ...state,
    visible: false,
  };
}

export function useIdleControls({ active, idleMs }: IdleControlsOptions) {
  const [idleState, setIdleState] = useState<IdleControlsState>(() => ({
    active,
    idleMs,
    visible: true,
  }));
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  let currentState = idleState;
  if (currentState.active !== active || currentState.idleMs !== idleMs) {
    currentState = {
      active,
      idleMs,
      visible: true,
    };
    setIdleState(currentState);
  }

  const showAndReset = () => {
    clearIdleTimer(timerRef);

    if (active) {
      timerRef.current = setTimeout(() => {
        timerRef.current = null;
        setIdleState(state => hideIdleControls(state, active, idleMs));
      }, idleMs);
    }

    setIdleState(state => {
      if (state.active === active && state.idleMs === idleMs && state.visible) {
        return state;
      }

      return {
        active,
        idleMs,
        visible: true,
      };
    });
  };

  useEffect(() => {
    clearIdleTimer(timerRef);

    if (!active) {
      return () => clearIdleTimer(timerRef);
    }

    timerRef.current = setTimeout(() => {
      timerRef.current = null;
      setIdleState(state => hideIdleControls(state, active, idleMs));
    }, idleMs);

    return () => clearIdleTimer(timerRef);
  }, [active, idleMs]);

  const visible = active ? currentState.visible : true;

  return { visible, showAndReset };
}
