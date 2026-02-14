import { formatTimeSeconds } from "@/lib/format";

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
      "absolute top-1/2 h-4 w-4 -translate-y-1/2 rounded-full bg-white opacity-0 shadow-lg transition-opacity group-hover:opacity-100 group-focus:opacity-100",
    timeText: "text-sm text-slate-400 tabular-nums",
    showTimes: true,
    timesLayout: "below",
  },
  minimized: {
    container: "hidden max-w-md flex-1 items-center gap-3 sm:flex",
    bar: "group relative h-1.5 flex-1 cursor-pointer rounded-full bg-slate-700 focus:ring-2 focus:ring-amber-400 focus:outline-none",
    thumb:
      "absolute top-1/2 h-3 w-3 -translate-y-1/2 rounded-full bg-white opacity-0 shadow-md transition-opacity group-hover:opacity-100 group-focus:opacity-100",
    timeText: "w-10 text-xs text-slate-400 tabular-nums",
    showTimes: true,
    timesLayout: "inline",
  },
  mobile: {
    container: "mt-2 sm:hidden",
    bar: "h-1 cursor-pointer rounded-full bg-slate-700 focus:ring-2 focus:ring-amber-400 focus:outline-none",
    thumb: "", // No thumb on mobile for cleaner look
    timeText: "text-xs text-slate-400 tabular-nums",
    showTimes: true,
    timesLayout: "below",
  },
  video: {
    container: "mb-4 w-full",
    bar: "group relative h-2 cursor-pointer rounded-full bg-slate-700 focus:ring-2 focus:ring-cyan-400 focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none",
    thumb:
      "absolute top-1/2 h-4 w-4 -translate-y-1/2 rounded-full bg-white opacity-0 shadow-lg transition-opacity group-hover:opacity-100 group-focus:opacity-100",
    timeText: "text-sm text-slate-400 tabular-nums",
    showTimes: true,
    timesLayout: "below",
  },
};

export default function ProgressBar({
  currentTime,
  duration,
  onSeek,
  variant,
}: ProgressBarProps) {
  const styles = variantStyles[variant];
  const progress = duration > 0 ? (currentTime / duration) * 100 : 0;

  const handleClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (!duration) return;

    const rect = e.currentTarget.getBoundingClientRect();
    const clickX = e.clientX - rect.left;
    const percentage = clickX / rect.width;
    const newTime = percentage * duration;

    onSeek(newTime);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!duration) return;

    if (e.key === "ArrowLeft") {
      e.preventDefault();
      onSeek(Math.max(0, currentTime - 5));
    } else if (e.key === "ArrowRight") {
      e.preventDefault();
      onSeek(Math.min(duration, currentTime + 5));
    }
  };

  // Calculate thumb offset based on variant
  const thumbOffset = variant === "minimized" ? 6 : 8;

  // Render inline layout (minimized desktop)
  if (styles.timesLayout === "inline") {
    return (
      <div
        className={styles.container}
        role="group"
        aria-label="Playback progress"
      >
        <span className={`${styles.timeText} text-right`} aria-hidden="true">
          {formatTimeSeconds(currentTime)}
        </span>

        <div
          onClick={handleClick}
          onKeyDown={handleKeyDown}
          tabIndex={0}
          className={styles.bar}
          role="slider"
          aria-label="Seek through track"
          aria-valuenow={Math.round(currentTime)}
          aria-valuemin={0}
          aria-valuemax={Math.round(duration)}
          aria-valuetext={`${formatTimeSeconds(currentTime)} of ${formatTimeSeconds(duration)}`}
        >
          <div
            className={
              variant === "video"
                ? "absolute inset-y-0 left-0 rounded-full bg-cyan-400 transition-all"
                : "absolute inset-y-0 left-0 rounded-full bg-amber-400 transition-all"
            }
            style={{ width: `${progress}%` }}
          />
          {styles.thumb && (
            <div
              className={styles.thumb}
              style={{ left: `calc(${progress}% - ${thumbOffset}px)` }}
            />
          )}
        </div>

        <span className={styles.timeText} aria-hidden="true">
          {formatTimeSeconds(duration)}
        </span>
      </div>
    );
  }

  // Render below layout (expanded, mobile, video)
  return (
    <div
      className={styles.container}
      role="group"
      aria-label="Playback progress"
    >
      <div
        onClick={handleClick}
        onKeyDown={handleKeyDown}
        tabIndex={0}
        className={styles.bar}
        role="slider"
        aria-label="Seek through track"
        aria-valuenow={Math.round(currentTime)}
        aria-valuemin={0}
        aria-valuemax={Math.round(duration)}
        aria-valuetext={`${formatTimeSeconds(currentTime)} of ${formatTimeSeconds(duration)}`}
      >
        <div
          className={
            variant === "mobile"
              ? "h-full rounded-full bg-amber-400 transition-all"
              : variant === "video"
                ? "absolute inset-y-0 left-0 rounded-full bg-cyan-400 transition-all"
                : "absolute inset-y-0 left-0 rounded-full bg-amber-400 transition-all"
          }
          style={{ width: `${progress}%` }}
        />
        {styles.thumb && (
          <div
            className={styles.thumb}
            style={{ left: `calc(${progress}% - ${thumbOffset}px)` }}
          />
        )}
      </div>
      {styles.showTimes && (
        <div
          className={
            variant === "mobile"
              ? "mt-1 flex justify-between"
              : "mt-2 flex justify-between"
          }
        >
          <span className={styles.timeText} aria-hidden="true">
            {formatTimeSeconds(currentTime)}
          </span>
          <span className={styles.timeText} aria-hidden="true">
            {formatTimeSeconds(duration)}
          </span>
        </div>
      )}
    </div>
  );
}
