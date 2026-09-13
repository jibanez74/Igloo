import { cn } from "@/lib/utils";

type WatchProgressBarProps = {
  progressSec: number;
  durationSec: number;
  /** Track colour; defaults suit a muted surface, heroes pass a light one. */
  trackClassName?: string;
  className?: string;
};

/**
 * Thin "how far in" strip shared by the movie hero and the episode rows. It is
 * decoration for sighted users; callers pair it with text ("12 min left") that
 * carries the same information for everyone else.
 */
export default function WatchProgressBar({
  progressSec,
  durationSec,
  trackClassName = "bg-muted",
  className,
}: WatchProgressBarProps) {
  const progressPct =
    durationSec > 0
      ? Math.min(100, Math.max(0, (progressSec / durationSec) * 100))
      : 0;

  return (
    <div
      className={cn(
        "h-1 w-full overflow-hidden rounded-full",
        trackClassName,
        className,
      )}
      aria-hidden="true"
    >
      <div
        className="h-full rounded-full bg-primary"
        style={{ width: `${progressPct}%` }}
      />
    </div>
  );
}
