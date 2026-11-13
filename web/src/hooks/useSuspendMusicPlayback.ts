import { useEffect } from "react";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";

/**
 * For a page that plays video (the video player, a trailer, a watch room):
 * pauses the music player when the page mounts and hands its keyboard
 * shortcuts to the page until it unmounts. Music stays paused afterwards; the
 * reader resumes it. Runs on mount, before the page's own media loads, so a
 * loading or failed video never plays over music either.
 */
export function useSuspendMusicPlayback() {
  const { pause, suspendKeyboard, resumeKeyboard } = useAudioPlayerActions();

  useEffect(() => {
    pause();
    suspendKeyboard();
    return () => resumeKeyboard();
  }, [pause, suspendKeyboard, resumeKeyboard]);
}
