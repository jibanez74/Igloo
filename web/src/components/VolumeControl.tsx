import { useState, useEffect } from "react";
import { Volume, Volume1, Volume2, VolumeX } from "lucide-react";

type MediaElement = HTMLAudioElement | HTMLVideoElement;

type VolumeControlProps = {
  mediaRef: React.RefObject<MediaElement | null>;
  variant?: "expanded" | "minimized";
  /** Accent color for focus ring and slider. "amber" for music, "cyan" for movies. */
  accent?: "amber" | "cyan";
};

const accentStyles = {
  amber: {
    focusRing: "focus:ring-amber-400",
    slider: "accent-amber-400",
  },
  cyan: {
    focusRing: "focus:ring-cyan-400",
    slider: "accent-cyan-400",
  },
} as const;

export default function VolumeControl({
  mediaRef,
  variant = "minimized",
  accent = "amber",
}: VolumeControlProps) {
  const [volume, setVolume] = useState(1);
  const [isMuted, setIsMuted] = useState(false);
  const [previousVolume, setPreviousVolume] = useState(1);

  const styles = accentStyles[accent];

  // Sync with media element (audio or video)
  useEffect(() => {
    const media = mediaRef.current;
    if (!media) return;

    const handleVolumeChange = () => {
      setVolume(media.volume);
      setIsMuted(media.muted);
    };

    media.addEventListener("volumechange", handleVolumeChange);
    setVolume(media.volume);
    setIsMuted(media.muted);

    return () => media.removeEventListener("volumechange", handleVolumeChange);
  }, [mediaRef]);

  const handleVolumeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newVolume = parseFloat(e.target.value);
    const media = mediaRef.current;
    if (media) {
      media.volume = newVolume;
      media.muted = false;
      setVolume(newVolume);
      setIsMuted(false);
    }
  };

  const toggleMute = () => {
    const media = mediaRef.current;
    if (!media) return;
    if (isMuted) {
      media.muted = false;
      media.volume = previousVolume || 0.5;
    } else {
      setPreviousVolume(volume);
      media.muted = true;
    }
  };

  const isExpanded = variant === "expanded";
  const iconClassName = isExpanded ? "size-5" : "size-4";

  const getVolumeIcon = () => {
    if (isMuted || volume === 0) return <VolumeX className={iconClassName} />;
    if (volume < 0.3) return <Volume className={iconClassName} />;
    if (volume < 0.7) return <Volume1 className={iconClassName} />;
    return <Volume2 className={iconClassName} />;
  };

  return (
    <div
      className={`flex items-center gap-2 ${isExpanded ? "w-32" : "group relative"}`}
      role="group"
      aria-label="Volume control"
    >
      <button
        onClick={toggleMute}
        className={`flex items-center justify-center rounded-full text-slate-400 transition-colors hover:text-white focus:ring-2 focus:outline-none ${styles.focusRing} ${
          isExpanded ? "h-10 w-10" : "size-8 hover:bg-slate-800"
        }`}
        aria-label={isMuted ? "Unmute" : "Mute"}
      >
        {getVolumeIcon()}
      </button>

      {/* Volume slider - always visible in expanded, hover in minimized */}
      <div
        className={
          isExpanded
            ? "flex-1"
            : "absolute bottom-full left-1/2 mb-2 hidden -translate-x-1/2 rounded-lg bg-slate-800 p-2 shadow-lg group-hover:block"
        }
      >
        <input
          type="range"
          min="0"
          max="1"
          step="0.01"
          value={isMuted ? 0 : volume}
          onChange={handleVolumeChange}
          className={`${styles.slider} ${
            isExpanded
              ? "h-1.5 w-full cursor-pointer appearance-none rounded-full bg-slate-700"
              : "h-20 w-1.5 cursor-pointer appearance-none rounded-full bg-slate-700"
          }`}
          style={
            isExpanded
              ? undefined
              : { writingMode: "vertical-lr", direction: "rtl" }
          }
          aria-label="Volume"
          aria-valuenow={Math.round((isMuted ? 0 : volume) * 100)}
          aria-valuemin={0}
          aria-valuemax={100}
        />
      </div>
    </div>
  );
}
