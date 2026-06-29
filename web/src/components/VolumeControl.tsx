import {
  useEffect,
  useEffectEvent,
  useId,
  useReducer,
  useRef,
  useState,
} from "react";
import { Volume, Volume1, Volume2, VolumeX } from "lucide-react";
import {
  MOTION_PLAYER_CHROME_BUTTON_CLASS,
  MOTION_PLAYER_CHROME_PANEL_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

type MediaElement = HTMLAudioElement | HTMLVideoElement;

type VolumeControlProps = {
  mediaRef: React.RefObject<MediaElement | null>;
  variant?: "expanded" | "minimized";
  /**
   * Caller hint for which surface the control sits on ("amber" = music,
   * "cyan" = movies). Both now resolve to the glacier primary accent.
   */
  accent?: "amber" | "cyan";
};

const accentStyles = {
  amber: {
    focusRing: "focus:ring-ring",
    slider: "accent-primary",
  },
  cyan: {
    focusRing: "focus:ring-ring",
    slider: "accent-primary",
  },
} as const;

type VolumeState = {
  volume: number;
  isMuted: boolean;
};

type VolumeAction = {
  type: "sync";
  volume: number;
  isMuted: boolean;
};

const initialVolumeState: VolumeState = {
  volume: 1,
  isMuted: false,
};

function volumeReducer(state: VolumeState, action: VolumeAction): VolumeState {
  if (state.volume === action.volume && state.isMuted === action.isMuted) {
    return state;
  }

  return {
    volume: action.volume,
    isMuted: action.isMuted,
  };
}

export default function VolumeControl({
  mediaRef,
  variant = "minimized",
  accent = "amber",
}: VolumeControlProps) {
  const [{ volume, isMuted }, dispatchVolume] = useReducer(
    volumeReducer,
    initialVolumeState,
  );
  const [isMinimizedPanelOpen, setIsMinimizedPanelOpen] = useState(false);

  const styles = accentStyles[accent];
  const controlId = useId();
  const containerRef = useRef<HTMLDivElement | null>(null);
  const triggerButtonRef = useRef<HTMLButtonElement | null>(null);
  const minimizedMuteButtonRef = useRef<HTMLButtonElement | null>(null);
  const sliderRef = useRef<HTMLInputElement | null>(null);
  const previousVolumeRef = useRef(1);
  const isExpanded = variant === "expanded";
  const currentVolume = isMuted ? 0 : volume;
  const volumePercent = Math.round(currentVolume * 100);
  const panelId = `${controlId}-volume-panel`;

  // Sync with media element (audio or video)
  useEffect(() => {
    const media = mediaRef.current;
    if (!media) return;

    const handleVolumeChange = () => {
      dispatchVolume({
        type: "sync",
        volume: media.volume,
        isMuted: media.muted,
      });
    };

    media.addEventListener("volumechange", handleVolumeChange);
    handleVolumeChange();

    return () => media.removeEventListener("volumechange", handleVolumeChange);
  }, [mediaRef]);

  const handleVolumeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const newVolume = parseFloat(e.target.value);
    const media = mediaRef.current;
    if (media) {
      media.volume = newVolume;
      media.muted = false;
      dispatchVolume({
        type: "sync",
        volume: newVolume,
        isMuted: false,
      });
    }
  };

  const toggleMute = () => {
    const media = mediaRef.current;
    if (!media) return;
    if (isMuted) {
      media.muted = false;
      media.volume = previousVolumeRef.current || 0.5;
    } else {
      previousVolumeRef.current = volume;
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
      minimizedMuteButtonRef.current?.focus();
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
      className={`${styles.slider} h-1.5 w-full cursor-pointer appearance-none rounded-full bg-muted`}
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
          className={cn(
            MOTION_PLAYER_CHROME_BUTTON_CLASS,
            "flex size-10 items-center justify-center rounded-full text-muted-foreground hover:text-foreground focus:ring-2 focus:outline-none",
            styles.focusRing,
          )}
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
        className={cn(
          MOTION_PLAYER_CHROME_BUTTON_CLASS,
          "flex size-8 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground focus:ring-2 focus:outline-none",
          styles.focusRing,
          isMinimizedPanelOpen && "bg-accent text-foreground",
        )}
        aria-label="Adjust volume"
        aria-controls={panelId}
        aria-expanded={isMinimizedPanelOpen}
      >
        {getVolumeIcon()}
      </button>

      {isMinimizedPanelOpen && (
        <div
          id={panelId}
          role="group"
          aria-label="Volume controls"
          className={cn(
            MOTION_PLAYER_CHROME_PANEL_CLASS,
            "absolute right-0 bottom-full z-10 mb-2 w-40 rounded-lg border border-border bg-background/95 p-3 shadow-lg backdrop-blur-sm",
          )}
        >
          <div className="mb-3 flex items-center justify-between gap-3">
            <button
              ref={minimizedMuteButtonRef}
              type="button"
              onClick={toggleMute}
              className={cn(
                MOTION_PLAYER_CHROME_BUTTON_CLASS,
                "flex size-8 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground focus:ring-2 focus:outline-none",
                styles.focusRing,
              )}
              aria-label={isMuted ? "Unmute" : "Mute"}
            >
              {getVolumeIcon()}
            </button>
            <span className="text-xs text-muted-foreground tabular-nums">
              {volumePercent}%
            </span>
          </div>
          {slider}
        </div>
      )}
    </div>
  );
}
