import { useState, useEffect } from "react";
import { Volume, Volume1, Volume2, VolumeX } from "lucide-react";

type VolumeControlProps = {
  audioRef: React.RefObject<HTMLAudioElement | null>;
  variant?: "expanded" | "minimized";
};

export default function VolumeControl({
  audioRef,
  variant = "minimized",
}: VolumeControlProps) {
  const [volume, setVolume] = useState(1);
  const [isMuted, setIsMuted] = useState(false);
  const [previousVolume, setPreviousVolume] = useState(1);

  // Sync with audio element
  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) return;

    const handleVolumeChange = () => {
      setVolume(audio.volume);
      setIsMuted(audio.muted);
    };

    audio.addEventListener("volumechange", handleVolumeChange);
    // Initialize
    setVolume(audio.volume);
    setIsMuted(audio.muted);

    return () => audio.removeEventListener("volumechange", handleVolumeChange);
  }, [audioRef]);

  const handleVolumeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newVolume = parseFloat(e.target.value);
    if (audioRef.current) {
      audioRef.current.volume = newVolume;
      audioRef.current.muted = false;
      setVolume(newVolume);
      setIsMuted(false);
    }
  };

  const toggleMute = () => {
    if (audioRef.current) {
      if (isMuted) {
        audioRef.current.muted = false;
        audioRef.current.volume = previousVolume || 0.5;
      } else {
        setPreviousVolume(volume);
        audioRef.current.muted = true;
      }
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
        className={`flex items-center justify-center rounded-full text-slate-400 transition-colors hover:text-white focus:ring-2 focus:ring-amber-400 focus:outline-none ${
          isExpanded ? "h-10 w-10" : "h-8 w-8 hover:bg-slate-800"
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
          className={`accent-amber-400 ${
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
