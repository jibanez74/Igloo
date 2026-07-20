import { useAudioPlayerState } from "@/hooks/useAudioPlayerState";

// Shared "is this row the current track" wiring for track lists — every list
// renders TrackItem with the same two props derived from the player state.
export function useTrackPlaybackMatcher() {
  const { currentTrack, isPlaying } = useAudioPlayerState();

  return (trackId: number) => {
    const isCurrentTrack = currentTrack?.id === trackId;

    return {
      isCurrentTrack,
      isPlaying: isCurrentTrack && isPlaying,
    };
  };
}
