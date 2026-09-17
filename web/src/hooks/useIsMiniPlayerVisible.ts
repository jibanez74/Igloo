import { useContext } from "react";
import { AudioPlayerNowPlayingContext } from "@/context/AudioPlayerContext";

// Whether the docked mini bar is on screen, so app chrome can leave room for
// it. Reads the now-playing context (not the queue) so growing an endless
// queue never re-renders the app shell. Tolerates a missing provider: some
// route tests render chrome without the player.
export function useIsMiniPlayerVisible() {
  const nowPlaying = useContext(AudioPlayerNowPlayingContext);

  return (
    nowPlaying !== null &&
    nowPlaying.currentTrackId !== null &&
    !nowPlaying.isExpanded
  );
}
