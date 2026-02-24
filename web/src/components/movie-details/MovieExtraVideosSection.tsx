import { Play } from "lucide-react";
import type { MovieExtraVideoView } from "@/lib/movie-details-view";

const YOUTUBE_WATCH_URL = "https://www.youtube.com/watch?v=";

type MovieExtraVideosSectionProps = {
  extraVideos: MovieExtraVideoView[];
};

export default function MovieExtraVideosSection({
  extraVideos,
}: MovieExtraVideosSectionProps) {
  const trailers = extraVideos.filter(
    v => v.type === "Trailer" && v.site === "YouTube",
  );
  const others = extraVideos.filter(
    v => !(v.type === "Trailer" && v.site === "YouTube"),
  );
  const hasTrailers = trailers.length > 0;
  const hasOthers = others.length > 0;
  if (!hasTrailers && !hasOthers) return null;

  return (
    <section className="mt-10 animate-in fade-in" aria-labelledby="extra-videos-heading">
      <h2
        id="extra-videos-heading"
        className="mb-4 text-xl font-semibold text-white"
        tabIndex={-1}
      >
        Extra Videos
      </h2>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        {hasTrailers &&
          trailers.map(video => (
            <a
              key={video.id}
              href={`${YOUTUBE_WATCH_URL}${video.key}`}
              target="_blank"
              rel="noopener noreferrer"
              className="flex min-h-11 min-w-0 items-center gap-3 rounded-lg border border-amber-500/20 bg-slate-800/50 px-4 py-3 transition-colors hover:border-amber-500/40 hover:bg-slate-800/80 focus-visible:ring-2 focus-visible:ring-amber-400/50"
            >
              <Play className="size-4 shrink-0 text-amber-400" aria-hidden="true" />
              <span className="min-w-0 truncate text-sm font-medium text-white">
                {video.title}
              </span>
              {video.official && (
                <span className="shrink-0 rounded-sm bg-amber-500/20 px-1.5 py-0.5 text-xs text-amber-300">
                  Official
                </span>
              )}
            </a>
          ))}
        {hasOthers &&
          others.map(video => (
            <div
              key={video.id}
              className="flex min-h-11 items-center gap-3 rounded-lg border border-slate-600/50 bg-slate-800/30 px-4 py-3"
            >
              <span className="truncate text-sm text-slate-300">
                {video.title}
              </span>
              <span className="shrink-0 text-xs text-slate-500">
                {video.type}
              </span>
            </div>
          ))}
      </div>
    </section>
  );
}
