import { useContext } from "react";
import { AudioPlayerStateContext } from "@/context/AudioPlayerContext";

export function useAudioPlayerState() {
  const state = useContext(AudioPlayerStateContext);

  if (!state) {
    throw new Error(
      "useAudioPlayerState must be used within an AudioPlayerProvider",
    );
  }

  return state;
}
