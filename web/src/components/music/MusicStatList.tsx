import type { ReactNode } from "react";
import { Clock, type LucideIcon } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { formatDuration } from "@/lib/format";

/** The labelled row of stat pills under an album or musician title. */
export default function MusicStatList({
  label,
  children,
}: {
  label: string;
  children: ReactNode;
}) {
  return (
    <ul
      className="mt-4 flex list-none flex-wrap items-center justify-center gap-2 sm:gap-3 lg:justify-start"
      aria-label={label}
    >
      {children}
    </ul>
  );
}

/** One stat pill: a muted icon beside its value, inside the list item. */
export function MusicStatChip({
  icon: Icon,
  children,
}: {
  icon: LucideIcon;
  children: ReactNode;
}) {
  return (
    <li>
      <Badge
        variant="outline"
        className="gap-1.5 border-border/40 bg-muted/90 px-3 py-1.5 text-sm font-normal text-foreground"
      >
        <Icon className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
        {children}
      </Badge>
    </li>
  );
}

/**
 * The total-duration pill. `aria-label` does not reliably name a `<time>`
 * (§1.7), so the spoken words are an sr-only span and the formatted value is
 * hidden from assistive tech.
 */
export function MusicDurationChip({ ms }: { ms: number }) {
  const duration = formatDuration(ms);

  return (
    <MusicStatChip icon={Clock}>
      <span className="sr-only">Total duration {duration}</span>
      <time dateTime={`PT${Math.round(ms / 1000)}S`} aria-hidden="true">
        {duration}
      </time>
    </MusicStatChip>
  );
}
