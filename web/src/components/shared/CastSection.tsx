import { User } from "lucide-react";
import ScrollRail from "@/components/shared/ScrollRail";
import {
  DETAIL_RAIL_HEADING_CLASS,
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_MICRO_COLORS_CLASS,
  TMDB_PROFILE_SIZE,
} from "@/lib/constants";
import { pluralize } from "@/lib/format";
import { cn } from "@/lib/utils";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";

/**
 * One billed role. `key` is explicit rather than derived from a person id
 * because TMDB aggregate credits can give one artist several roles on the same
 * show: movies pass the cast row id, shows pass the credit id, and keying on
 * the artist would duplicate React keys.
 */
export type CastSectionItem = {
  key: string;
  name: string;
  character: string;
  profilePath: string | null;
  /** Only shows have a per-actor episode tally. */
  episodeCount?: number | null;
};

type CastSectionProps = {
  cast: CastSectionItem[];
};

const CAST_DISPLAY_LIMIT = 10;

export default function CastSection({ cast }: CastSectionProps) {
  if (!cast || cast.length === 0) {
    return null;
  }

  const displayedCast = cast.slice(0, CAST_DISPLAY_LIMIT);

  return (
    <section className="mt-8 sm:mt-10" aria-labelledby="cast-heading">
      <h2
        id="cast-heading"
        className={DETAIL_RAIL_HEADING_CLASS}
        tabIndex={-1}
      >
        Cast
      </h2>

      {displayedCast.length < cast.length && (
        <p className="sr-only">
          Showing {displayedCast.length} of {cast.length} cast members.
        </p>
      )}

      {/* Focusable so keyboard users can scroll the horizontal strip; role
          kept because Tailwind's list-none strips list semantics in Safari. */}
      <ScrollRail label="cast" asChild>
        <ul
          tabIndex={0}
          className={cn("list-none gap-3 pb-4 sm:gap-4", FOCUS_VISIBLE_RING_CLASS)}
          role="list"
          aria-label={`Cast members, ${displayedCast.length} shown`}
        >
        {displayedCast.map(actor => {
          const episodeLabel =
            actor.episodeCount != null && actor.episodeCount > 0
              ? pluralize(actor.episodeCount, "episode")
              : null;

          return (
            <li
              key={actor.key}
              className={cn(
                MOTION_MICRO_COLORS_CLASS,
                "w-32 shrink-0 overflow-hidden rounded-lg border border-primary/20 bg-muted/50 hover:border-primary/40",
              )}
            >
              <article
                aria-label={
                  episodeLabel
                    ? `${actor.name} as ${actor.character}, ${episodeLabel}`
                    : `${actor.name} as ${actor.character}`
                }
              >
                {actor.profilePath ? (
                  // The article already speaks the name; a named photo would
                  // read it twice.
                  <img
                    src={buildTmdbImageUrl(actor.profilePath, TMDB_PROFILE_SIZE)}
                    alt=""
                    className="aspect-2/3 w-full object-cover"
                    loading="lazy"
                  />
                ) : (
                  <div
                    className="flex aspect-2/3 w-full items-center justify-center bg-accent"
                    role="img"
                    aria-label={`No photo available for ${actor.name}`}
                  >
                    <User
                      className="size-6 text-muted-foreground"
                      aria-hidden="true"
                    />
                  </div>
                )}
                <div className="p-2">
                  <p className="truncate text-sm font-semibold text-foreground">
                    {actor.name}
                  </p>
                  {/* The article above already speaks name, role, and tally, so
                      these lines are decorative to a screen reader. */}
                  <p
                    className="truncate text-xs text-muted-foreground"
                    aria-hidden="true"
                  >
                    {actor.character}
                  </p>
                  {episodeLabel && (
                    <p
                      className="truncate text-xs text-muted-foreground"
                      aria-hidden="true"
                    >
                      {episodeLabel}
                    </p>
                  )}
                </div>
              </article>
            </li>
          );
        })}
        </ul>
      </ScrollRail>
    </section>
  );
}
