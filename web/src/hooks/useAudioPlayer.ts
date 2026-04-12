import { useMemo } from "react";
import type { AudioPlayerContextType } from "@/types";
import { useAudioPlayerActions } from "./useAudioPlayerActions";
import { useAudioPlayerState } from "./useAudioPlayerState";

export function useAudioPlayer(): AudioPlayerContextType {
  const state = useAudioPlayerState();
  const actions = useAudioPlayerActions();

  return useMemo(
    () => ({
      ...state,
      ...actions,
    }),
    [state, actions],
  );
}
