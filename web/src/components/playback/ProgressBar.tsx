import { useState } from "react";
import { formatSpokenTime, formatTimecode } from "@/lib/format";
import {
  CARD_FOCUS_WITHIN_RING_CLASS,
  MOTION_PROGRESS_FILL_CLASS,
  MOTION_PROGRESS_THUMB_REVEAL_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

type ProgressBarVariant =
  | "expanded"
  | "minimized"
  | "mobile"
  | "video"
  | "trailer";

type ProgressBarProps = {
  currentTime: number;
  duration: number;
  onSeek: (newTime: number) => void;
  variant: ProgressBarVariant;
  ariaLabel?: string;
  resetKey?: string | number;
};

// Names the slider's wrapping group for assistive technology.
const GROUP_LABEL = "Playback progress";

// Variant-specific styles
const variantStyles: Record<
  ProgressBarVariant,
  {
    container: string;
    bar: string;
    thumb: string;
    timeText: string;
    showTimes: boolean;
    timesLayout: "below" | "inline";
  }
> = (() => {
  // Shared pieces the variants compose. All bars carry the whole-group
  // focus-within variant of the single ring recipe (design-system §1.7);
  // thumbs are bg-foreground — every variant sits on a themed panel, so the
  // over-media white/black exception (§1.2) does not apply here.
  const barBase = cn("relative rounded-full bg-muted", CARD_FOCUS_WITHIN_RING_CLASS);
  const thumbBase =
    "absolute top-1/2 -translate-x-1/2 -translate-y-1/2 rounded-full bg-foreground opacity-0 group-hover:opacity-100 group-focus-within:opacity-100";
  const timeTextBase = "text-muted-foreground tabular-nums";

  const expanded = {
    container: "mb-6 w-full max-w-md",
    bar: cn("group h-2", barBase),
    thumb: cn("size-4 shadow-lg", thumbBase),
    timeText: cn("text-sm", timeTextBase),
    showTimes: true,
    timesLayout: "below" as const,
  };

  return {
    expanded,
    minimized: {
      container: "hidden max-w-md flex-1 items-center gap-3 sm:flex",
      bar: cn("group h-1.5 flex-1", barBase),
      thumb: cn("size-3 shadow-md", thumbBase),
      timeText: cn("w-10 text-xs", timeTextBase),
      showTimes: true,
      timesLayout: "inline",
    },
    mobile: {
      container: "mt-2 sm:hidden",
      bar: cn("h-1", barBase),
      thumb: "", // No thumb on mobile for cleaner look
      timeText: cn("text-xs", timeTextBase),
      showTimes: true,
      timesLayout: "below",
    },
    video: { ...expanded, container: "mb-4 w-full" },
    trailer: {
      container: "mb-4 w-full",
      bar: cn("group h-1.5", barBase),
      thumb: cn("size-3 shadow-md", thumbBase),
      timeText: cn("text-sm", timeTextBase),
      showTimes: false,
      timesLayout: "below",
    },
  };
})();

function clampToRange(value: number, min: number, max: number) {
  if (!Number.isFinite(value)) {
    return min;
  }

  return Math.min(max, Math.max(min, value));
}

export default function ProgressBar({
  currentTime,
  duration,
  onSeek,
  variant,
  ariaLabel = "Seek through track",
  resetKey,
}: ProgressBarProps) {
  const styles = variantStyles[variant];
  // While the user is scrubbing, the media element's currentTime lags behind
  // (seeks are async), so the displayed position follows the pending scrub
  // value instead of snapping back to the stale currentTime prop.
  const [scrubTime, setScrubTime] = useState<number | null>(null);
  // When the playing media changes (e.g. track auto-advance mid-drag), a
  // pending scrub value belongs to the old media and must not carry over.
  const [prevResetKey, setPrevResetKey] = useState(resetKey);
  if (resetKey !== prevResetKey) {
    setPrevResetKey(resetKey);
    setScrubTime(null);
  }
  const safeDuration =
    Number.isFinite(duration) && duration > 0 ? duration : 0;
  const isSeekable = safeDuration > 0;
  const safeCurrentTime = isSeekable
    ? clampToRange(scrubTime ?? currentTime, 0, safeDuration)
    : 0;
  const progress = isSeekable ? (safeCurrentTime / safeDuration) * 100 : 0;
  const pageSeekStep = Math.min(30, Math.max(10, safeDuration * 0.1));
  const showThumb = Boolean(styles.thumb) && isSeekable;
  const fillClassName = cn(
    "absolute inset-y-0 left-0 rounded-full bg-primary",
    MOTION_PROGRESS_FILL_CLASS,
  );

  const seekTo = (nextTime: number) => {
    if (!isSeekable) {
      return;
    }

    onSeek(clampToRange(nextTime, 0, safeDuration));
  };

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    if (!isSeekable) {
      return;
    }

    const nextTime = clampToRange(e.target.valueAsNumber, 0, safeDuration);
    setScrubTime(nextTime);
    onSeek(nextTime);
  };

  const clearScrub = () => {
    setScrubTime(null);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!isSeekable) return;

    switch (e.key) {
      case "ArrowLeft":
      case "ArrowDown":
        e.preventDefault();
        seekTo(safeCurrentTime - 5);
        break;
      case "ArrowRight":
      case "ArrowUp":
        e.preventDefault();
        seekTo(safeCurrentTime + 5);
        break;
      case "Home":
        e.preventDefault();
        seekTo(0);
        break;
      case "End":
        e.preventDefault();
        seekTo(safeDuration);
        break;
      case "PageDown":
        e.preventDefault();
        seekTo(safeCurrentTime - pageSeekStep);
        break;
      case "PageUp":
        e.preventDefault();
        seekTo(safeCurrentTime + pageSeekStep);
        break;
    }
  };

  const slider = (
    <div className={cn(styles.bar, !isSeekable && "opacity-60")}>
      <div
        className={fillClassName}
        style={{ width: `${progress}%` }}
      />
      {showThumb && (
        <div
          className={cn(styles.thumb, MOTION_PROGRESS_THUMB_REVEAL_CLASS)}
          style={{ left: `${progress}%` }}
        />
      )}
      {/* A transparent native range input handles all interaction: unlike a
          custom role="slider" div, it stays operable with iOS VoiceOver's
          adjustable gestures, which never reach pointer or keydown handlers. */}
      <input
        type="range"
        min={0}
        max={isSeekable ? safeDuration : 0}
        step={1}
        value={safeCurrentTime}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        onPointerUp={clearScrub}
        onPointerCancel={clearScrub}
        onBlur={clearScrub}
        tabIndex={isSeekable ? 0 : -1}
        className={cn(
          "absolute top-1/2 left-0 h-6 w-full -translate-y-1/2 appearance-none bg-transparent opacity-0 focus:outline-hidden",
          isSeekable ? "cursor-pointer touch-none" : "cursor-default",
        )}
        aria-label={ariaLabel}
        aria-disabled={!isSeekable}
        aria-valuetext={
          isSeekable
            ? `${formatSpokenTime(safeCurrentTime)} of ${formatSpokenTime(safeDuration)}`
            : "Seek unavailable"
        }
      />
    </div>
  );

  const showHours = safeDuration >= 3600;
  const currentTimeLabel = formatTimecode(safeCurrentTime, {
    forceHours: showHours,
  });
  const durationLabel = formatTimecode(safeDuration);

  if (styles.timesLayout === "inline") {
    return (
      <div
        className={styles.container}
        role="group"
        aria-label={GROUP_LABEL}
      >
        <span className={`${styles.timeText} text-right`} aria-hidden="true">
          {currentTimeLabel}
        </span>
        {slider}
        <span className={styles.timeText} aria-hidden="true">
          {durationLabel}
        </span>
      </div>
    );
  }

  return (
    <div
      className={styles.container}
      role="group"
      aria-label={GROUP_LABEL}
    >
      {slider}
      {styles.showTimes && (
        <div
          className={
            variant === "mobile"
              ? "mt-1 flex justify-between"
              : "mt-2 flex justify-between"
          }
        >
          <span className={styles.timeText} aria-hidden="true">
            {currentTimeLabel}
          </span>
          <span className={styles.timeText} aria-hidden="true">
            {durationLabel}
          </span>
        </div>
      )}
    </div>
  );
}
