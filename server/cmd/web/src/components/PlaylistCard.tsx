import { Link } from "@tanstack/react-router";
import { ListMusic } from "lucide-react";
import { unwrapString } from "@/lib/nullable";
import type { PlaylistSummaryType } from "@/types";
import { formatDuration } from "@/lib/format";

type PlaylistCardProps = {
  playlist: PlaylistSummaryType;
};

export default function PlaylistCard({ playlist }: PlaylistCardProps) {
  const { id, name, track_count, total_duration, cover_image, is_owner } =
    playlist;
  const coverUrl = unwrapString(cover_image);

  return (
    <article className="group relative animate-in overflow-hidden rounded-xl border border-slate-800 bg-slate-900 p-4 transition-all duration-300 fade-in hover:-translate-y-1 hover:border-amber-400/50 hover:shadow-xl hover:shadow-amber-400/20">
      <Link
        to="/music/playlist/$id"
        params={{ id: id.toString() }}
        className="block focus:ring-2 focus:ring-amber-400 focus:outline-none focus:ring-inset"
        aria-label={`${name}, ${track_count} tracks, ${formatDuration(total_duration)}`}
      >
        {/* Playlist cover - square with aspect-square to prevent CLS */}
        <div className="relative mx-auto mb-3 aspect-square w-full overflow-hidden rounded-lg bg-slate-800">
          {coverUrl ? (
            <img
              src={coverUrl}
              alt={name}
              className="size-full object-cover transition-transform duration-300 group-hover:scale-105"
            />
          ) : (
            <div className="flex size-full items-center justify-center bg-linear-to-br from-slate-700 via-slate-800 to-cyan-900/30">
              <ListMusic className="size-10 text-cyan-200/30" aria-hidden="true" />
            </div>
          )}

          {/* Owner badge */}
          {is_owner && (
            <div className="absolute top-2 right-2 rounded-full bg-amber-500/90 px-2 py-0.5 text-xs font-medium text-slate-900">
              Owner
            </div>
          )}
        </div>

        {/* Playlist info */}
        <div className="text-center">
          <h3 className="truncate text-sm font-semibold text-white">{name}</h3>
          <p className="mt-0.5 text-xs text-slate-400">
            {track_count} {track_count === 1 ? "track" : "tracks"}
            {total_duration > 0 && ` · ${formatDuration(total_duration)}`}
          </p>
        </div>
      </Link>
    </article>
  );
}
