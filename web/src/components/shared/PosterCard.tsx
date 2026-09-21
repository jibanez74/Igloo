import { Link, type LinkProps } from "@tanstack/react-router";
import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";
import { Play } from "lucide-react";
import WatchProgressBar from "@/components/shared/WatchProgressBar";
import { usePosterFallback } from "@/hooks/usePosterFallback";
import {
  CARD_ACTION_REVEAL_CLASS,
  CARD_FOCUS_WITHIN_RING_CLASS,
  CARD_MEDIA_HOVER_CLASS,
  CARD_OVERLAY_REVEAL_CLASS,
  CARD_SURFACE_CLASS,
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_LOADING_STATE_CLASS,
} from "@/lib/constants";
import { cn } from "@/lib/utils";

export type PosterCardWatchProgress = {
  progressSec: number;
  durationSec: number;
};

/**
 * The play action travels with its label: the button is an icon alone, so
 * without `playLabel` it would reach a screen reader unnamed. A card with
 * nothing single to play (a show, an unreleased title) passes neither.
 */
type PosterCardPlayProps =
  | {
      /** The revealed play action, bypassing the details page. */
      playLink: LinkProps;
      playLabel: string;
    }
  | {
      playLink?: undefined;
      playLabel?: never;
    };

type PosterCardProps = {
  /** Where the poster itself navigates - the media's details page. */
  detailsLink: LinkProps;
  /** TMDB poster URL, or "" to show the fallback icon. */
  posterUrl: string;
  fallbackIcon: LucideIcon;
  title: string;
  /** Second line under the title - a year, or an episode code and name. */
  subtitle?: string;
  detailsLabel: string;
  watchProgress?: PosterCardWatchProgress;
  /** Decorative corner slot over the poster - a rating badge, say. */
  badge?: ReactNode;
  /** Warms the details query on hover and focus. */
  onPrefetch?: () => void;
} & PosterCardPlayProps;

/**
 * The 2:3 media card used across the home rows and library grids: poster,
 * optional corner badge, optional watch-progress bar, and an optional play
 * action that bypasses the details page. The hover wash and the play control
 * travel together - a card with nothing single to play (a show, an unreleased
 * title) renders neither. The percent is announced through the poster link's
 * label, so the bar itself stays decorative.
 */
export default function PosterCard({
  detailsLink,
  playLink,
  posterUrl,
  fallbackIcon: FallbackIcon,
  title,
  subtitle,
  detailsLabel,
  playLabel,
  watchProgress,
  badge,
  onPrefetch,
}: PosterCardProps) {
  const { showPoster, onError } = usePosterFallback(posterUrl);

  const hasProgress =
    watchProgress !== undefined && watchProgress.durationSec > 0;

  return (
    <article
      className={cn(CARD_SURFACE_CLASS, "min-w-0", CARD_FOCUS_WITHIN_RING_CLASS)}
      onMouseEnter={onPrefetch}
      onFocus={onPrefetch}
    >
      <div className="relative">
        <Link
          {...detailsLink}
          className={cn(
            "block rounded-xl outline-hidden",
            FOCUS_VISIBLE_RING_CLASS,
          )}
          aria-label={detailsLabel}
        >
          {/* Poster with 2:3 aspect ratio (standard poster) */}
          <div className="relative aspect-2/3 bg-muted">
            {showPoster ? (
              <img
                src={posterUrl}
                alt=""
                width={500}
                height={750}
                loading="lazy"
                decoding="async"
                fetchPriority="low"
                className={cn("size-full object-cover", CARD_MEDIA_HOVER_CLASS)}
                onError={onError}
              />
            ) : (
              <div className="flex size-full items-center justify-center">
                <FallbackIcon
                  className="size-10 text-muted-foreground"
                  aria-hidden="true"
                />
              </div>
            )}
            {/* Overlay - appears on hover/focus behind the play action. A
                card with nothing to play omits it rather than washing out on
                hover for no reason. */}
            {playLink && (
              <div
                className={cn(
                  CARD_OVERLAY_REVEAL_CLASS,
                  "absolute inset-0 flex items-center justify-center bg-black/30 opacity-0 group-focus-within:opacity-100 group-hover:opacity-100",
                )}
                aria-hidden="true"
              />
            )}
            {badge}
            {/* Gradient overlay for text readability */}
            <div className="absolute inset-x-0 bottom-0 h-28 bg-linear-to-t from-black/90 via-black/50 to-transparent" />
            {/* Watch progress bar - percent is announced via the link label */}
            {hasProgress && (
              <WatchProgressBar
                progressSec={watchProgress.progressSec}
                durationSec={watchProgress.durationSec}
                trackClassName="bg-white/25"
                className="absolute inset-x-0 bottom-0 rounded-none"
                fillClassName=""
              />
            )}
          </div>
          {/* Title block */}
          <div className="absolute inset-x-0 bottom-0 p-3">
            <h3 className="line-clamp-2 text-sm/tight font-semibold text-white drop-shadow-lg">
              {title}
            </h3>
            {subtitle && (
              <p className="mt-0.5 text-xs text-white/80 drop-shadow-lg">
                {subtitle}
              </p>
            )}
          </div>
        </Link>
      </div>

      {/* Play button - goes to the play page without opening details */}
      {playLink && (
        <Link
          {...playLink}
          className={cn(
            CARD_ACTION_REVEAL_CLASS,
            FOCUS_VISIBLE_RING_CLASS,
            "absolute top-1/2 left-1/2 z-10 flex size-14 -translate-1/2 scale-90 items-center justify-center rounded-full bg-primary text-primary-foreground opacity-0 shadow-lg shadow-black/30 outline-hidden group-focus-within:scale-100 group-focus-within:opacity-100 group-hover:scale-100 group-hover:opacity-100 hover:bg-primary/90 focus-visible:scale-100 focus-visible:opacity-100",
          )}
          aria-label={playLabel}
        >
          <Play className="size-7 fill-current" aria-hidden="true" />
        </Link>
      )}
    </article>
  );
}

// Authored beside the card it mirrors (design-system §3.4): the same 2:3 box
// and two text bars, for every grid that loads poster cards.
export function PosterCardSkeleton() {
  return (
    <div
      className={cn(
        "overflow-hidden rounded-xl border border-border bg-card",
        MOTION_LOADING_STATE_CLASS,
      )}
    >
      <div className="aspect-2/3 bg-muted" />
      <div className="p-3">
        <div className="h-4 w-3/4 rounded-sm bg-muted" />
        <div className="mt-2 h-3 w-1/2 rounded-sm bg-muted" />
      </div>
    </div>
  );
}
