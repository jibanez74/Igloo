import { BRAND_MARK_GLYPH_VIEWBOX, BRAND_MARK_PATH } from "@/lib/brand-mark";
import { cn } from "@/lib/utils";

type BrandMarkProps = {
  className?: string;
};

/**
 * The Igloo brand glyph (docs/design-system.md §1.1): the same igloo as the
 * app icons, cropped to its own bounds and drawn in `currentColor` with
 * transparent cut-outs so the parent decides the tile. Decorative — the surrounding link or heading names it.
 */
export default function BrandMark({ className }: BrandMarkProps) {
  return (
    <svg
      viewBox={BRAND_MARK_GLYPH_VIEWBOX}
      aria-hidden="true"
      focusable="false"
      className={cn("shrink-0", className)}
    >
      <path d={BRAND_MARK_PATH} fill="currentColor" fillRule="evenodd" />
    </svg>
  );
}
