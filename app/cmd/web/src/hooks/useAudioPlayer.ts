import { useContext } from "react";
import { AudioPlayerContext } from "@/context/AudioPlayerContext";

// Hook to access the audio player context
export function useAudioPlayer() {
  const context = useContext(AudioPlayerContext);
  if (!context) {
    throw new Error(
      "useAudioPlayer must be used within an AudioPlayerProvider"
    );
  }
  return context;
}
