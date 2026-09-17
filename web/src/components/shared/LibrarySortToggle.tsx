import { ArrowDownAZ, ArrowUpAZ } from "lucide-react";
import {
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_MICRO_CONTROL_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

export type LibrarySortDirection = "asc" | "desc";

type LibrarySortToggleProps = {
  sort: LibrarySortDirection;
  onToggle: () => void;
};

// The one sort control a library page has: a single button that flips between
// A–Z and Z–A. The accessible name states both the current order and what a
// click does, so it needs no visible hint.
export default function LibrarySortToggle({
  sort,
  onToggle,
}: LibrarySortToggleProps) {
  return (
    <button
      type="button"
      onClick={onToggle}
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full bg-muted px-3 py-1.5 text-sm font-medium text-muted-foreground hover:bg-accent hover:text-foreground",
        MOTION_MICRO_CONTROL_CLASS,
        FOCUS_VISIBLE_RING_CLASS,
      )}
      aria-label={
        sort === "asc"
          ? "Sorted A to Z, click to sort Z to A"
          : "Sorted Z to A, click to sort A to Z"
      }
    >
      {sort === "asc" ? (
        <>
          <ArrowDownAZ className="size-4" aria-hidden="true" />
          A–Z
        </>
      ) : (
        <>
          <ArrowUpAZ className="size-4" aria-hidden="true" />
          Z–A
        </>
      )}
    </button>
  );
}
