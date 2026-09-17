import { useAudioPlayerNowPlaying } from "@/hooks/useAudioPlayerNowPlaying";

// Shared "is this row the current track" wiring for track lists — every list
// renders TrackItem with the same two props derived from the player state.
// Reads the now-playing context (not the full player state) so rows don't
// re-render when the queue itself changes.
export function useTrackPlaybackMatcher() {
  const { currentTrackId, isPlaying } = useAudioPlayerNowPlaying();

  return (trackId: number) => {
    const isCurrentTrack = currentTrackId === trackId;

    return {
      isCurrentTrack,
      isPlaying: isCurrentTrack && isPlaying,
    };
  };
}
