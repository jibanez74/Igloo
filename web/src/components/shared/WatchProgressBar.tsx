import { watchProgressPercent } from "@/lib/format";
import { cn } from "@/lib/utils";

type WatchProgressBarProps = {
  progressSec: number;
  durationSec: number;
  /** Track colour; defaults suit a muted surface, heroes pass a light one. */
  trackClassName?: string;
  className?: string;
  /** Fill shape; poster cards sit flush on an edge and drop the rounding. */
  fillClassName?: string;
};

/**
 * Thin "how far in" strip shared by the movie hero, the episode rows and the
 * poster cards. It is decoration for sighted users; callers pair it with text
 * ("1 hr 35 min left", "25% watched") that carries the same information for
 * everyone else.
 */
export default function WatchProgressBar({
  progressSec,
  durationSec,
  trackClassName = "bg-muted",
  className,
  fillClassName = "rounded-full",
}: WatchProgressBarProps) {
  const progressPct = watchProgressPercent(progressSec, durationSec);

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
        className={cn("h-full bg-primary", fillClassName)}
        style={{ width: `${progressPct}%` }}
      />
    </div>
  );
}
