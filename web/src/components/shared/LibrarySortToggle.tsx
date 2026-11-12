import { ArrowDownAZ, ArrowUpAZ } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { SortOrder } from "@/types";

type LibrarySortToggleProps = {
  sort: SortOrder;
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
    <Button
      variant="outline"
      size="sm"
      shape="pill"
      onClick={onToggle}
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
    </Button>
  );
}
