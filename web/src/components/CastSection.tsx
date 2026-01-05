import {
  TMDB_IMAGE_BASE,
  TMDB_PROFILE_SIZE,
} from "@/lib/constants";

interface CastMember {
  id: number;
  name: string;
  character: string;
  profile_path: string;
  order: number;
}

interface CastSectionProps {
  cast: CastMember[];
  maxDisplay?: number;
}

export default function CastSection({ cast, maxDisplay = 10 }: CastSectionProps) {
  if (!cast || cast.length === 0) {
    return null;
  }

  const displayedCast = cast.slice(0, maxDisplay);

  return (
    <section className='mt-10' aria-labelledby='cast-heading'>
      <h2
        id='cast-heading'
        className='text-2xl font-semibold text-white mb-4'
        tabIndex={-1}
      >
        Top Billed Cast
      </h2>
      <p className='sr-only'>
        Showing {displayedCast.length} of {cast.length} cast members. Use arrow keys to scroll horizontally.
      </p>
      <ul
        className='flex gap-4 overflow-x-auto pb-4 -mx-4 px-4 scrollbar-thin scrollbar-thumb-cyan-700/50 list-none'
        role='list'
        aria-label={`Cast members, ${displayedCast.length} shown`}
      >
        {displayedCast.map((actor, index) => (
          <li
            key={actor.id}
            className='shrink-0 w-32 bg-slate-800/50 rounded-lg overflow-hidden border border-cyan-500/20 hover:border-cyan-500/40 focus-within:border-cyan-400 focus-within:ring-2 focus-within:ring-cyan-400/50 transition-colors'
          >
            <article
              tabIndex={0}
              role='article'
              aria-label={`${actor.name} as ${actor.character}`}
              aria-posinset={index + 1}
              aria-setsize={displayedCast.length}
              className='outline-none cursor-default'
            >
              {actor.profile_path ? (
                <img
                  src={`${TMDB_IMAGE_BASE}/${TMDB_PROFILE_SIZE}${actor.profile_path}`}
                  alt={`Photo of ${actor.name}`}
                  className='w-full aspect-2/3 object-cover'
                  loading='lazy'
                />
              ) : (
                <div
                  className='w-full aspect-2/3 bg-slate-700 flex items-center justify-center'
                  role='img'
                  aria-label={`No photo available for ${actor.name}`}
                >
                  <i className='fa-solid fa-user text-slate-500 text-2xl' aria-hidden='true' />
                </div>
              )}
              <div className='p-2'>
                <p className='font-semibold text-white text-sm truncate'>
                  {actor.name}
                </p>
                <p className='text-xs text-slate-400 truncate' aria-label={`Playing ${actor.character}`}>
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

