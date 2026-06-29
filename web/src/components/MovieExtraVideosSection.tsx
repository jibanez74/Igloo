import { Link } from "@tanstack/react-router";
import { formatExtraVideoType } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { MovieExtraVideosSectionProps } from "@/types";

export default function MovieExtraVideosSection({
  videos,
  movieId,
  trailerReturnTo,
}: MovieExtraVideosSectionProps) {
  if (videos.length === 0) return null;

  const returnTo = trailerReturnTo ?? `/movies/${movieId}`;

  return (
    <section
      className="mt-8 sm:mt-10"
      aria-labelledby="extra-videos-heading"
    >
      <h2
        id="extra-videos-heading"
        tabIndex={-1}
        className="mb-4 text-xl font-semibold text-white outline-none sm:text-2xl"
      >
        Extra Videos
      </h2>
      <p className="sr-only">
        Extra videos scroll horizontally. Use Tab to move between video links.
        On touch devices, swipe or scroll the list to see all clips.
      </p>
      <ul
        className="-mx-4 flex snap-x snap-mandatory scrollbar-thin scrollbar-thumb-amber-700/50 list-none gap-3 overflow-x-auto overscroll-x-contain px-4 pb-4 sm:-mx-6 sm:gap-4 sm:px-6 lg:-mx-8 lg:gap-4 lg:px-8"
        aria-label={`Extra videos, ${videos.length} clips`}
      >
        {videos.map(video => (
          <li
            key={video.id}
            className="w-[min(22rem,calc(100vw-2.5rem))] shrink-0 snap-start scroll-ms-1 scroll-me-1 sm:scroll-ms-2 sm:scroll-me-2"
          >
            <Link
              to="/trailer"
              search={{
                videoKey: video.key,
                returnTo,
              }}
              className={cn(
                "flex min-h-13 touch-manipulation flex-col justify-center rounded-lg border border-amber-500/20 bg-slate-800/80 px-3 py-2.5 text-left text-sm text-amber-200 transition-colors",
                "hover:border-amber-500/40 hover:bg-slate-800",
                "focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:outline-none",
                "sm:min-h-0",
              )}
            >
              <span className="leading-snug font-medium">{video.title}</span>
              <span className="mt-0.5 text-slate-400">
                ({formatExtraVideoType(video.type)})
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}
