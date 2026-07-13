import { useState } from "react";
import { formatSpokenTime, formatTimecode } from "@/lib/format";
import {
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
  groupLabel?: string;
  resetKey?: string | number;
};

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
> = {
  expanded: {
    container: "mb-6 w-full max-w-md",
    bar: "group relative h-2 rounded-full bg-muted focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2 focus-within:ring-offset-background",
    thumb:
      "absolute top-1/2 h-4 w-4 -translate-x-1/2 -translate-y-1/2 rounded-full bg-white opacity-0 shadow-lg group-hover:opacity-100 group-focus-within:opacity-100",
    timeText: "text-sm text-muted-foreground tabular-nums",
    showTimes: true,
    timesLayout: "below",
  },
  minimized: {
    container: "hidden max-w-md flex-1 items-center gap-3 sm:flex",
    bar: "group relative h-1.5 flex-1 rounded-full bg-muted focus-within:ring-2 focus-within:ring-ring",
    thumb:
      "absolute top-1/2 h-3 w-3 -translate-x-1/2 -translate-y-1/2 rounded-full bg-white opacity-0 shadow-md group-hover:opacity-100 group-focus-within:opacity-100",
    timeText: "w-10 text-xs text-muted-foreground tabular-nums",
    showTimes: true,
    timesLayout: "inline",
  },
  mobile: {
    container: "mt-2 sm:hidden",
    bar: "relative h-1 rounded-full bg-muted focus-within:ring-2 focus-within:ring-ring",
    thumb: "", // No thumb on mobile for cleaner look
    timeText: "text-xs text-muted-foreground tabular-nums",
    showTimes: true,
    timesLayout: "below",
  },
  video: {
    container: "mb-4 w-full",
    bar: "group relative h-2 rounded-full bg-muted focus-within:ring-2 focus-within:ring-ring focus-within:ring-offset-2 focus-within:ring-offset-background",
    thumb:
      "absolute top-1/2 h-4 w-4 -translate-x-1/2 -translate-y-1/2 rounded-full bg-white opacity-0 shadow-lg group-hover:opacity-100 group-focus-within:opacity-100",
    timeText: "text-sm text-muted-foreground tabular-nums",
    showTimes: true,
    timesLayout: "below",
  },
  trailer: {
    container: "mb-4 w-full",
    bar: "group relative h-1.5 rounded-full bg-muted focus-within:ring-2 focus-within:ring-ring",
    thumb:
      "absolute top-1/2 size-3 -translate-x-1/2 -translate-y-1/2 rounded-full bg-white opacity-0 shadow-md group-hover:opacity-100 group-focus-within:opacity-100",
    timeText: "text-sm text-muted-foreground tabular-nums",
    showTimes: false,
    timesLayout: "below",
  },
};

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
  groupLabel = "Playback progress",
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
          "absolute top-1/2 left-0 h-6 w-full -translate-y-1/2 appearance-none bg-transparent opacity-0 focus:outline-none",
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
        aria-label={groupLabel}
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
      aria-label={groupLabel}
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
