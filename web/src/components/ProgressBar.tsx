import { useRef } from "react";
import { formatTimeSeconds } from "@/lib/format";
import { cn } from "@/lib/utils";

type ProgressBarVariant = "expanded" | "minimized" | "mobile" | "video";

type ProgressBarProps = {
  currentTime: number;
  duration: number;
  onSeek: (newTime: number) => void;
  variant: ProgressBarVariant;
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
    bar: "group relative h-2 cursor-pointer rounded-full bg-slate-700 focus:ring-2 focus:ring-amber-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none",
    thumb:
      "absolute top-1/2 h-4 w-4 -translate-x-1/2 -translate-y-1/2 rounded-full bg-white opacity-0 shadow-lg transition-opacity group-hover:opacity-100 group-focus:opacity-100",
    timeText: "text-sm text-slate-400 tabular-nums",
    showTimes: true,
    timesLayout: "below",
  },
  minimized: {
    container: "hidden max-w-md flex-1 items-center gap-3 sm:flex",
    bar: "group relative h-1.5 flex-1 cursor-pointer rounded-full bg-slate-700 focus:ring-2 focus:ring-amber-400 focus:outline-none",
    thumb:
      "absolute top-1/2 h-3 w-3 -translate-x-1/2 -translate-y-1/2 rounded-full bg-white opacity-0 shadow-md transition-opacity group-hover:opacity-100 group-focus:opacity-100",
    timeText: "w-10 text-xs text-slate-400 tabular-nums",
    showTimes: true,
    timesLayout: "inline",
  },
  mobile: {
    container: "mt-2 sm:hidden",
    bar: "relative h-1 cursor-pointer rounded-full bg-slate-700 focus:ring-2 focus:ring-amber-400 focus:outline-none",
    thumb: "", // No thumb on mobile for cleaner look
    timeText: "text-xs text-slate-400 tabular-nums",
    showTimes: true,
    timesLayout: "below",
  },
  video: {
    container: "mb-4 w-full",
    bar: "group relative h-2 cursor-pointer rounded-full bg-slate-700 focus:ring-2 focus:ring-cyan-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none",
    thumb:
      "absolute top-1/2 h-4 w-4 -translate-x-1/2 -translate-y-1/2 rounded-full bg-white opacity-0 shadow-lg transition-opacity group-hover:opacity-100 group-focus:opacity-100",
    timeText: "text-sm text-slate-400 tabular-nums",
    showTimes: true,
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
}: ProgressBarProps) {
  const styles = variantStyles[variant];
  const activePointerIdRef = useRef<number | null>(null);
  const safeDuration =
    Number.isFinite(duration) && duration > 0 ? duration : 0;
  const isSeekable = safeDuration > 0;
  const safeCurrentTime = isSeekable
    ? clampToRange(currentTime, 0, safeDuration)
    : 0;
  const progress = isSeekable ? (safeCurrentTime / safeDuration) * 100 : 0;
  const pageSeekStep = Math.min(30, Math.max(10, safeDuration * 0.1));
  const showThumb = Boolean(styles.thumb) && isSeekable;
  const fillClassName =
    variant === "video"
      ? "absolute inset-y-0 left-0 rounded-full bg-cyan-400 transition-all"
      : "absolute inset-y-0 left-0 rounded-full bg-amber-400 transition-all";

  const seekTo = (nextTime: number) => {
    if (!isSeekable) {
      return;
    }

    onSeek(clampToRange(nextTime, 0, safeDuration));
  };

  const getSeekTime = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!isSeekable) {
      return 0;
    }

    const rect = e.currentTarget.getBoundingClientRect();
    if (rect.width <= 0) {
      return 0;
    }

    const x = Math.max(0, Math.min(e.clientX - rect.left, rect.width));
    return (x / rect.width) * safeDuration;
  };

  const handlePointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!isSeekable) return;

    activePointerIdRef.current = e.pointerId;
    e.currentTarget.setPointerCapture(e.pointerId);
    seekTo(getSeekTime(e));
  };

  const handlePointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!isSeekable || activePointerIdRef.current !== e.pointerId) return;

    seekTo(getSeekTime(e));
  };

  const releasePointerDrag = (e: React.PointerEvent<HTMLDivElement>) => {
    const activePointerId = activePointerIdRef.current;
    if (activePointerId !== null && e.currentTarget.hasPointerCapture(activePointerId)) {
      e.currentTarget.releasePointerCapture(activePointerId);
    }
    activePointerIdRef.current = null;
  };

  const handlePointerUp = (e: React.PointerEvent<HTMLDivElement>) => {
    if (isSeekable && activePointerIdRef.current === e.pointerId) {
      seekTo(getSeekTime(e));
    }

    releasePointerDrag(e);
  };

  const handlePointerCancel = (e: React.PointerEvent<HTMLDivElement>) => {
    if (activePointerIdRef.current !== e.pointerId) {
      return;
    }

    releasePointerDrag(e);
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
    <div
      onPointerDown={handlePointerDown}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerUp}
      onPointerCancel={handlePointerCancel}
      onKeyDown={handleKeyDown}
      tabIndex={isSeekable ? 0 : -1}
      className={cn(
        styles.bar,
        isSeekable && "touch-none",
        !isSeekable && "cursor-default opacity-60",
      )}
      role="slider"
      aria-label="Seek through track"
      aria-disabled={!isSeekable}
      aria-orientation="horizontal"
      aria-valuenow={Math.round(safeCurrentTime)}
      aria-valuemin={0}
      aria-valuemax={Math.round(safeDuration)}
      aria-valuetext={
        isSeekable
          ? `${formatTimeSeconds(safeCurrentTime)} of ${formatTimeSeconds(safeDuration)}`
          : "Seek unavailable"
      }
    >
      <div
        className={fillClassName}
        style={{ width: `${progress}%` }}
      />
      {showThumb && (
        <div
          className={styles.thumb}
          style={{ left: `${progress}%` }}
        />
      )}
    </div>
  );

  const currentTimeLabel = formatTimeSeconds(safeCurrentTime);
  const durationLabel = formatTimeSeconds(safeDuration);

  if (styles.timesLayout === "inline") {
    return (
      <div
        className={styles.container}
        role="group"
        aria-label="Playback progress"
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
      aria-label="Playback progress"
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
