import { Link } from "@tanstack/react-router";
import { formatTimeSeconds } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { MovieChaptersSectionProps } from "@/types";

export default function MovieChaptersSection({
  chapters,
  movieId,
  playbackSettings,
}: MovieChaptersSectionProps) {
  if (chapters.length === 0) return null;

  return (
    <section className="mt-8 sm:mt-10" aria-labelledby="chapters-heading">
      <h2
        id="chapters-heading"
        tabIndex={-1}
        className="mb-3 text-lg font-semibold text-white outline-none sm:text-xl md:text-2xl"
      >
        Chapters
      </h2>
      <p className="sr-only">
        Chapters scroll horizontally. Use Tab to move between chapter links. On
        touch devices, swipe or scroll the list to see all chapters.
      </p>
      <ul
        className="scrollbar-thin scrollbar-thumb-amber-700/50 -mx-4 flex snap-x snap-mandatory list-none gap-3 overflow-x-auto overscroll-x-contain px-4 pb-4 sm:-mx-6 sm:gap-4 sm:px-6 lg:-mx-8 lg:gap-4 lg:px-8"
        aria-label={`Chapters, ${chapters.length} total`}
      >
        {chapters.map(chapter => (
          <li
            key={chapter.id}
            className="w-[min(18rem,calc(100vw-2.5rem))] shrink-0 snap-start scroll-ms-1 scroll-me-1 sm:scroll-ms-2 sm:scroll-me-2"
          >
            <Link
              to="/movies/$id/play"
              params={{ id: String(movieId) }}
              search={{
                mode: playbackSettings.mode,
                audio_track: playbackSettings.audioTrack,
                subtitle_track: playbackSettings.subtitleTrack ?? undefined,
                start: chapter.start_time,
              }}
              className={cn(
                "flex min-h-13 touch-manipulation flex-col justify-center rounded-lg border border-amber-500/20 bg-slate-800/80 px-3 py-2.5 text-left text-sm text-amber-200 transition-colors",
                "hover:border-amber-500/40 hover:bg-slate-800",
                "focus-visible:border-amber-400 focus-visible:ring-2 focus-visible:ring-amber-400/60 focus-visible:outline-none",
                "sm:min-h-0",
              )}
            >
              <span className="leading-snug font-medium">{chapter.title}</span>
              <span className="mt-0.5 text-slate-400">
                {formatTimeSeconds(chapter.start_time)}
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}
