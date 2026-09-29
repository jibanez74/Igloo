import { useEffect, useRef, useState } from "react";
import { clearTimeoutRef } from "@/lib/utils";

type IdleControlsOptions = {
  active: boolean;
  idleMs: number;
};

type IdleControlsState = {
  active: boolean;
  idleMs: number;
  visible: boolean;
};

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
  const timerRef = useRef<number | null>(null);

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
    clearTimeoutRef(timerRef);

    if (active) {
      timerRef.current = window.setTimeout(() => {
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
    clearTimeoutRef(timerRef);

    if (!active) {
      return () => clearTimeoutRef(timerRef);
    }

    timerRef.current = window.setTimeout(() => {
      timerRef.current = null;
      setIdleState(state => hideIdleControls(state, active, idleMs));
    }, idleMs);

    return () => clearTimeoutRef(timerRef);
  }, [active, idleMs]);

  const visible = active ? currentState.visible : true;

  return { visible, showAndReset };
}
