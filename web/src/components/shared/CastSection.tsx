import { User } from "lucide-react";
import {
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_MICRO_COLORS_CLASS,
  TMDB_PROFILE_SIZE,
} from "@/lib/constants";
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
  maxDisplay?: number;
};

export default function CastSection({
  cast,
  maxDisplay = 10,
}: CastSectionProps) {
  if (!cast || cast.length === 0) {
    return null;
  }

  const displayedCast = cast.slice(0, maxDisplay);

  return (
    <section className="mt-8 sm:mt-10" aria-labelledby="cast-heading">
      <h2
        id="cast-heading"
        className="mb-4 text-xl font-semibold text-foreground outline-hidden sm:text-2xl"
        tabIndex={-1}
      >
        Cast
      </h2>

      <p className="sr-only">
        Showing {displayedCast.length} of {cast.length} cast members. Scroll
        horizontally to see more.
      </p>

      {/* Focusable so keyboard users can scroll the horizontal strip; role
          kept because Tailwind's list-none strips list semantics in Safari. */}
      <ul
        tabIndex={0}
        className={cn(
          "-mx-4 flex scrollbar-thin scrollbar-thumb-primary/50 list-none gap-3 overflow-x-auto px-4 pb-4 sm:-mx-6 sm:gap-4 sm:px-6 lg:-mx-8 lg:px-8",
          FOCUS_VISIBLE_RING_CLASS,
        )}
        role="list"
        aria-label={`Cast members, ${displayedCast.length} shown`}
      >
        {displayedCast.map(actor => {
          const episodeLabel =
            actor.episodeCount != null && actor.episodeCount > 0
              ? `${actor.episodeCount} ${actor.episodeCount === 1 ? "episode" : "episodes"}`
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
                  <img
                    src={buildTmdbImageUrl(actor.profilePath, TMDB_PROFILE_SIZE)}
                    alt={actor.name}
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
                      className="truncate text-xs text-muted-foreground/80"
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
    </section>
  );
}
