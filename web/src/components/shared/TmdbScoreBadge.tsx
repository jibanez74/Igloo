import type { ComponentProps } from "react";
import { Badge } from "@/components/ui/badge";

type TmdbScoreBadgeProps = Omit<ComponentProps<"span">, "children"> & {
  score: number;
  /** `sm` fits a poster card's corner; the default sits in a hero's chip row. */
  size?: "default" | "sm";
  /** `over-poster` darkens the chip over raw poster art (§1.6). */
  variant?: "over-media" | "over-poster";
};

/**
 * TMDB's community score as a quiet labelled badge: the one rating chip in
 * the app, on heroes and poster corners alike (design-system §3.2, and §1.7
 * for the spoken value). A caller whose label already speaks the score
 * (`InTheatersCard`) passes `aria-hidden`.
 */
export default function TmdbScoreBadge({
  score,
  size = "default",
  variant = "over-media",
  className,
  ...props
}: TmdbScoreBadgeProps) {
  return (
    <Badge
      variant={variant}
      size={size === "sm" ? "sm" : "lg"}
      className={className}
      {...props}
    >
      <span className="sr-only">
        {`TMDB user score: ${score.toFixed(1)} out of 10`}
      </span>
      <span aria-hidden="true" className="text-white/70">
        TMDB
      </span>
      <span aria-hidden="true" className="font-semibold">
        {score.toFixed(1)}
      </span>
    </Badge>
  );
}
