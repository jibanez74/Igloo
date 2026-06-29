import { User } from "lucide-react";
import { TMDB_PROFILE_SIZE } from "@/lib/constants";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import type { CastMemberType } from "@/types";

type CastSectionProps = {
  cast: CastMemberType[];
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
        className="mb-4 text-xl font-semibold text-white outline-none sm:text-2xl"
        tabIndex={-1}
      >
        Cast
      </h2>

      <p className="sr-only">
        Showing {displayedCast.length} of {cast.length} cast members. Use Tab to
        move between cast cards, or scroll horizontally to see more.
      </p>

      <ul
        className="-mx-4 flex scrollbar-thin scrollbar-thumb-amber-700/50 list-none gap-3 overflow-x-auto px-4 pb-4 sm:-mx-6 sm:gap-4 sm:px-6 lg:-mx-8 lg:px-8"
        role="list"
        aria-label={`Cast members, ${displayedCast.length} shown`}
      >
        {displayedCast.map((actor, index) => (
          <li
            key={actor.id}
            className="w-32 shrink-0 overflow-hidden rounded-lg border border-amber-500/20 bg-slate-800/50 transition-colors focus-within:border-ring focus-within:ring-2 focus-within:ring-ring/50 hover:border-amber-500/40"
          >
            <article
              tabIndex={0}
              role="article"
              aria-label={`${actor.name} as ${actor.character}`}
              aria-posinset={index + 1}
              aria-setsize={displayedCast.length}
              className="cursor-default outline-none"
            >
              {actor.profile_path ? (
                <img
                  src={buildTmdbImageUrl(actor.profile_path, TMDB_PROFILE_SIZE)}
                  alt={`Photo of ${actor.name}`}
                  className="aspect-2/3 w-full object-cover"
                  loading="lazy"
                />
              ) : (
                <div
                  className="flex aspect-2/3 w-full items-center justify-center bg-slate-700"
                  role="img"
                  aria-label={`No photo available for ${actor.name}`}
                >
                  <User className="size-6 text-slate-400" aria-hidden="true" />
                </div>
              )}
              <div className="p-2">
                <p className="truncate text-sm font-semibold text-white">
                  {actor.name}
                </p>
                <p
                  className="truncate text-xs text-slate-400"
                  aria-label={`Playing ${actor.character}`}
                >
                  {actor.character}
                </p>
              </div>
            </article>
          </li>
        ))}
      </ul>
    </section>
  );
}
