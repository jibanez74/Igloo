import { useContext } from "react";
import { AudioPlayerActionsContext } from "@/context/audio-player-contexts";

export function useAudioPlayerActions() {
  const actions = useContext(AudioPlayerActionsContext);

  if (!actions) {
    throw new Error(
      "useAudioPlayerActions must be used within an AudioPlayerProvider",
    );
  }

  return actions;
}
