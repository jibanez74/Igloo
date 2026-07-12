import { useContext } from "react";
import { AudioPlayerStateContext } from "@/context/AudioPlayerContext";

export function useIsMiniPlayerVisible() {
  const playerState = useContext(AudioPlayerStateContext);

  return (
    playerState !== null &&
    playerState.currentTrack !== null &&
    !playerState.isExpanded
  );
}
