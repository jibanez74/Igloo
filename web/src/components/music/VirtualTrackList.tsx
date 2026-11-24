import { useEffect, type CSSProperties, type ReactNode } from "react";
import { useWindowVirtualizer, type Virtualizer } from "@tanstack/react-virtual";
import { useWindowScrollMargin } from "@/hooks/useWindowScrollMargin";
import { VIRTUAL_LIST_TRACK_HEIGHT } from "@/lib/constants";
import { cn } from "@/lib/utils";

// The divider a divided list draws under every row but the last, as the
// album page's divide-y does; the row's slot grows by its pixel.
const ROW_DIVIDER_CLASS = "border-b border-border/30";

type VirtualTrackListProps = {
  /** Rows to lay out: the tracks, and any headers placed between them. */
  count: number;
  /** The list's accessible name. */
  label: string;
  /** How many tracks the whole list holds, loaded or not. */
  setSize: number;
  /**
   * A row's 1-based place among the tracks, or null for a row that is not a
   * track (a section header). Defaults to the row's own position.
   */
  positionOf?: (index: number) => number | null;
  /** A row's height. Defaults to a track row's. */
  estimateSize?: (index: number) => number;
  /**
   * A row's identity, so a row keeps its element when the list above it
   * changes. Defaults to its position.
   */
  getItemKey?: (index: number) => string | number;
  /** Draws a hairline between rows. */
  divided?: boolean;
  renderRow: (index: number) => ReactNode;
  /** A paged list's infinite loader (useVirtualizedInfiniteLoader). */
  onChange?: (virtualizer: Virtualizer<Window, Element>) => void;
  /** The frame around the list. */
  className?: string;
  /** Under the rows, outside the list: a paged list's load-more footer. */
  footer?: ReactNode;
};

const trackRowHeight = () => VIRTUAL_LIST_TRACK_HEIGHT;
const ownPosition = (index: number) => index + 1;

/**
 * A track list that mounts only the rows near the viewport, for lists that
 * can run to thousands of tracks: the library's Tracks tab, a long
 * playlist, a musician's every track. The app shell scrolls the window, so
 * this is a window virtualizer offset by the list's place on the page.
 *
 * The sizing box is the list itself, so the rows are its direct items and a
 * footer stays outside it. Only a window of rows exists, so each track row
 * says where it sits in the whole list. Tailwind's preflight drops <ul>
 * semantics in Safari/VoiceOver; the explicit roles keep them.
 */
export default function VirtualTrackList({
  count,
  label,
  setSize,
  positionOf = ownPosition,
  estimateSize = trackRowHeight,
  getItemKey,
  divided = false,
  renderRow,
  onChange,
  className,
  footer,
}: VirtualTrackListProps) {
  // The virtualizer is a mutable object read during render, which the React
  // Compiler would cache stale.
  "use no memo";

  const { listRef, scrollMargin } = useWindowScrollMargin();
  const hasDivider = (index: number) => divided && index < count - 1;

  const virtualizer = useWindowVirtualizer({
    count,
    estimateSize: (index) => estimateSize(index) + (hasDivider(index) ? 1 : 0),
    getItemKey,
    overscan: 5,
    scrollMargin,
    onChange,
  });

  useEffect(() => {
    virtualizer.measure();
  }, [count, scrollMargin, virtualizer]);

  return (
    <div ref={listRef} className={className}>
      {/* react-doctor-disable-next-line react-doctor/prefer-tag-over-role */}
      <div
        role="list"
        aria-label={label}
        className="relative h-(--list-height) w-full"
        style={
          {
            "--list-height": `${virtualizer.getTotalSize()}px`,
          } as CSSProperties
        }
      >
        {virtualizer.getVirtualItems().map((virtualRow) => {
          const position = positionOf(virtualRow.index);

          return (
            <div
              key={virtualRow.key}
              role={position === null ? "presentation" : "listitem"}
              aria-posinset={position ?? undefined}
              aria-setsize={position === null ? undefined : setSize}
              className={cn(
                "absolute top-0 left-0 h-(--row-height) w-full translate-y-(--row-y)",
                hasDivider(virtualRow.index) && ROW_DIVIDER_CLASS,
              )}
              style={
                {
                  "--row-height": `${virtualRow.size}px`,
                  "--row-y": `${virtualRow.start - scrollMargin}px`,
                } as CSSProperties
              }
            >
              {renderRow(virtualRow.index)}
            </div>
          );
        })}
      </div>

      {footer}
    </div>
  );
}
