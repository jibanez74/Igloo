import { Link } from "@tanstack/react-router";
import { ArrowLeft } from "lucide-react";
import {
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_MICRO_COLORS_CLASS,
} from "@/lib/constants";
import type { MusicSearchParams } from "@/lib/route-search";
import { cn } from "@/lib/utils";

type MusicDetailBackNavProps = {
  /** The music library tab this page came from. */
  tab: MusicSearchParams["tab"];
  /** Visible link text, e.g. "Back to Albums"; the accessible name adds " library". */
  label: string;
  className?: string;
};

// The way back from an album, musician or playlist page to its library tab.
// One landmark so every music detail page ends the same way.
export default function MusicDetailBackNav({
  tab,
  label,
  className,
}: MusicDetailBackNavProps) {
  return (
    <nav className={className} aria-label="Page navigation">
      <Link
        to="/music"
        search={{ tab }}
        className={cn(
          MOTION_MICRO_COLORS_CLASS,
          FOCUS_VISIBLE_RING_CLASS,
          "inline-flex items-center gap-2 rounded-md px-2 py-1 text-muted-foreground hover:text-primary",
        )}
        aria-label={`${label} library`}
      >
        <ArrowLeft className="size-4" aria-hidden="true" />
        {label}
      </Link>
    </nav>
  );
}
