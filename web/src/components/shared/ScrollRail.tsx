import {
  useEffect,
  useRef,
  useState,
  type ReactNode,
  type RefObject,
} from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";
import { Slot } from "radix-ui";
import { Button } from "@/components/ui/button";
import { usePrefersCoarsePointer } from "@/hooks/use-coarse-pointer";
import {
  SCROLL_RAIL_ARROW_CLASS,
  SCROLL_RAIL_BLEED_CLASS,
  SCROLL_RAIL_EDGE_CLASS,
  SCROLL_RAIL_SCROLLER_CLASS,
} from "@/lib/constants";
import { getPrefersReducedMotion } from "@/lib/motion";
import { cn } from "@/lib/utils";

type ScrollRailProps = {
  /** Lowercase noun for the arrow names: "Scroll cast left". */
  label: string;
  /**
   * Make the single child the scroller instead of wrapping it in a div, so a
   * `<ul>` keeps its list role, label and focusability.
   */
  asChild?: boolean;
  /** Extra scroller classes: gaps, snap, bottom padding, a focus ring. */
  className?: string;
  /**
   * Where the edges stop above the scroller's bottom padding, so the fades
   * never cover the scrollbar. Mirrors the scroller's `pb-*`.
   */
  edgeInsetClassName?: string;
  children: ReactNode;
};

type ScrollEdges = { start: boolean; end: boolean };

const SCROLL_PAGE_FRACTION = 0.8;
// Sub-pixel scroll positions would otherwise flicker an edge on and off.
const EDGE_TOLERANCE_PX = 1;

/**
 * Which sides of a horizontal scroller still have content past the edge.
 * Re-measured on scroll, on any resize of the scroller or its items, and on
 * window resize, so an item that loads in later reveals its edge too.
 */
function useScrollEdges(ref: RefObject<HTMLElement | null>): ScrollEdges {
  const [edges, setEdges] = useState<ScrollEdges>({ start: false, end: false });

  useEffect(() => {
    const el = ref.current;
    if (!el) return;

    const measure = () => {
      const start = el.scrollLeft > EDGE_TOLERANCE_PX;
      const end =
        el.scrollLeft + el.clientWidth < el.scrollWidth - EDGE_TOLERANCE_PX;
      setEdges(prev =>
        prev.start === start && prev.end === end ? prev : { start, end },
      );
    };

    measure();

    const observer = new ResizeObserver(measure);
    observer.observe(el);
    for (const child of el.children) {
      observer.observe(child);
    }
    el.addEventListener("scroll", measure, { passive: true });
    window.addEventListener("resize", measure);

    return () => {
      observer.disconnect();
      el.removeEventListener("scroll", measure);
      window.removeEventListener("resize", measure);
    };
  }, [ref]);

  return edges;
}

/**
 * A horizontal rail on the shell's viewport bleed (design-system §3.2) that
 * says when there is more: an edge fade on each side that still overflows,
 * and prev/next arrows that appear on hover and on focus-within. The arrows
 * are left out on a touch-first device, where swiping is the affordance and
 * two more tab stops would only be noise.
 *
 * Both arrows stay rendered while either side overflows; the exhausted one is
 * `aria-disabled` and inert rather than removed, so a keyboard user who pages
 * to the end does not lose focus with the button.
 */
export default function ScrollRail({
  label,
  asChild = false,
  className,
  edgeInsetClassName = "bottom-4",
  children,
}: ScrollRailProps) {
  // Typed as a div ref for the plain scroller; with `asChild` the Slot hands
  // it the child element instead, which the edge math only needs as an
  // HTMLElement.
  const scrollerRef = useRef<HTMLDivElement | null>(null);
  const { start, end } = useScrollEdges(scrollerRef);
  const coarsePointer = usePrefersCoarsePointer();
  const Scroller = asChild ? Slot.Root : "div";
  const overflows = start || end;
  const showArrows = overflows && !coarsePointer;

  const scrollByPage = (direction: -1 | 1, enabled: boolean) => {
    const el = scrollerRef.current;
    if (!el || !enabled) return;
    el.scrollBy({
      left: direction * el.clientWidth * SCROLL_PAGE_FRACTION,
      behavior: getPrefersReducedMotion() ? "auto" : "smooth",
    });
  };

  const renderEdge = (side: "start" | "end") => {
    const canScroll = side === "start" ? start : end;
    if (!canScroll && !showArrows) return null;
    const Icon = side === "start" ? ChevronLeft : ChevronRight;
    const direction = side === "start" ? "left" : "right";

    return (
      <div
        data-rail-edge={side}
        data-overflow={canScroll || undefined}
        className={cn(
          SCROLL_RAIL_EDGE_CLASS,
          edgeInsetClassName,
          side === "start"
            ? "left-0 justify-start pl-1"
            : "right-0 justify-end pr-1",
          canScroll && (side === "start" ? "bg-linear-to-r" : "bg-linear-to-l"),
        )}
      >
        {showArrows && (
          <Button
            type="button"
            variant="outline"
            size="icon-sm"
            className={SCROLL_RAIL_ARROW_CLASS}
            aria-label={`Scroll ${label} ${direction}`}
            aria-disabled={!canScroll || undefined}
            onClick={() => scrollByPage(side === "start" ? -1 : 1, canScroll)}
          >
            <Icon aria-hidden="true" />
          </Button>
        )}
      </div>
    );
  };

  return (
    <div className={SCROLL_RAIL_BLEED_CLASS}>
      <Scroller
        ref={scrollerRef}
        className={cn(SCROLL_RAIL_SCROLLER_CLASS, className)}
      >
        {children}
      </Scroller>
      {renderEdge("start")}
      {renderEdge("end")}
    </div>
  );
}
