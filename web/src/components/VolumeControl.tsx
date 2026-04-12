import { useEffect, useEffectEvent, useId, useRef, useState } from "react";
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
  const [isMinimizedPanelOpen, setIsMinimizedPanelOpen] = useState(false);

  const styles = accentStyles[accent];
  const controlId = useId();
  const containerRef = useRef<HTMLDivElement | null>(null);
  const triggerButtonRef = useRef<HTMLButtonElement | null>(null);
  const sliderRef = useRef<HTMLInputElement | null>(null);
  const isExpanded = variant === "expanded";
  const currentVolume = isMuted ? 0 : volume;
  const volumePercent = Math.round(currentVolume * 100);
  const panelId = `${controlId}-volume-panel`;

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

  const iconClassName = isExpanded ? "size-5" : "size-4";

  const getVolumeIcon = () => {
    if (isMuted || volume === 0) return <VolumeX className={iconClassName} />;
    if (volume < 0.3) return <Volume className={iconClassName} />;
    if (volume < 0.7) return <Volume1 className={iconClassName} />;
    return <Volume2 className={iconClassName} />;
  };

  const closeMinimizedPanel = useEffectEvent((focusTrigger?: boolean) => {
    setIsMinimizedPanelOpen(false);

    if (focusTrigger) {
      triggerButtonRef.current?.focus();
    }
  });

  const handleDocumentPointerDown = useEffectEvent((event: PointerEvent) => {
    if (!containerRef.current?.contains(event.target as Node)) {
      closeMinimizedPanel();
    }
  });

  const handleDocumentFocusIn = useEffectEvent((event: FocusEvent) => {
    if (!containerRef.current?.contains(event.target as Node)) {
      closeMinimizedPanel();
    }
  });

  const handleDocumentKeyDown = useEffectEvent((event: KeyboardEvent) => {
    if (event.key === "Escape") {
      event.preventDefault();
      closeMinimizedPanel(true);
    }
  });

  useEffect(() => {
    if (isExpanded || !isMinimizedPanelOpen) {
      return;
    }

    document.addEventListener("pointerdown", handleDocumentPointerDown);
    document.addEventListener("focusin", handleDocumentFocusIn);
    document.addEventListener("keydown", handleDocumentKeyDown);

    return () => {
      document.removeEventListener("pointerdown", handleDocumentPointerDown);
      document.removeEventListener("focusin", handleDocumentFocusIn);
      document.removeEventListener("keydown", handleDocumentKeyDown);
    };
  }, [isExpanded, isMinimizedPanelOpen]);

  useEffect(() => {
    if (!isExpanded && isMinimizedPanelOpen) {
      sliderRef.current?.focus();
    }
  }, [isExpanded, isMinimizedPanelOpen]);

  const slider = (
    <input
      ref={sliderRef}
      type="range"
      min="0"
      max="1"
      step="0.01"
      value={currentVolume}
      onChange={handleVolumeChange}
      className={`${styles.slider} h-1.5 w-full cursor-pointer appearance-none rounded-full bg-slate-700`}
      aria-label="Volume"
      aria-valuenow={volumePercent}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-valuetext={`${volumePercent}% volume`}
    />
  );

  if (isExpanded) {
    return (
      <div className="flex w-32 items-center gap-2" role="group" aria-label="Volume control">
        <button
          type="button"
          onClick={toggleMute}
          className={`flex size-10 items-center justify-center rounded-full text-slate-400 transition-colors hover:text-white focus:ring-2 focus:outline-none ${styles.focusRing}`}
          aria-label={isMuted ? "Unmute" : "Mute"}
        >
          {getVolumeIcon()}
        </button>

        <div className="flex-1">{slider}</div>
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      className="relative"
      role="group"
      aria-label="Volume control"
    >
      <button
        ref={triggerButtonRef}
        type="button"
        onClick={() => setIsMinimizedPanelOpen(open => !open)}
        className={`flex size-8 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:outline-none ${styles.focusRing} ${
          isMinimizedPanelOpen && "bg-slate-800 text-white"
        }`}
        aria-label="Adjust volume"
        aria-controls={panelId}
        aria-expanded={isMinimizedPanelOpen}
        aria-haspopup="dialog"
      >
        {getVolumeIcon()}
      </button>

      {isMinimizedPanelOpen && (
        <div
          id={panelId}
          role="dialog"
          aria-label="Volume controls"
          className="absolute right-0 bottom-full z-10 mb-2 w-40 rounded-lg border border-slate-700 bg-slate-900/95 p-3 shadow-lg backdrop-blur-sm"
        >
          <div className="mb-3 flex items-center justify-between gap-3">
            <button
              type="button"
              onClick={toggleMute}
              className={`flex size-8 items-center justify-center rounded-full text-slate-400 transition-colors hover:bg-slate-800 hover:text-white focus:ring-2 focus:outline-none ${styles.focusRing}`}
              aria-label={isMuted ? "Unmute" : "Mute"}
            >
              {getVolumeIcon()}
            </button>
            <span className="text-xs text-slate-400 tabular-nums">
              {volumePercent}%
            </span>
          </div>
          {slider}
        </div>
      )}
    </div>
  );
}
