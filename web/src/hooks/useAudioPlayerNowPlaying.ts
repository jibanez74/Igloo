import { useContext } from "react";
import { AudioPlayerNowPlayingContext } from "@/context/audio-player-contexts";

export function useAudioPlayerNowPlaying() {
  const nowPlaying = useContext(AudioPlayerNowPlayingContext);

  if (!nowPlaying) {
    throw new Error(
      "useAudioPlayerNowPlaying must be used within an AudioPlayerProvider",
    );
  }

  return nowPlaying;
}
