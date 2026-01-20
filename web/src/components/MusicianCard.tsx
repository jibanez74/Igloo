import { Link } from "@tanstack/react-router";
import { User } from "lucide-react";
import type { SimpleMusicianType } from "@/types";

type MusicianCardProps = {
  musician: SimpleMusicianType;
};

export default function MusicianCard({ musician }: MusicianCardProps) {
  const { id, name, thumb, album_count, track_count } = musician;
  const thumbUrl = thumb?.Valid ? thumb.String : null;

  return (
    <article className="group relative animate-in overflow-hidden rounded-xl border border-slate-800 bg-slate-900 p-4 transition-all duration-300 fade-in hover:-translate-y-1 hover:border-amber-400/50 hover:shadow-xl hover:shadow-amber-400/20">
      <Link
        to="/music/musician/$id"
        params={{ id: id.toString() }}
        className="block focus:ring-2 focus:ring-amber-400 focus:outline-none focus:ring-inset"
        aria-label={`${name}, ${album_count} albums, ${track_count} tracks`}
      >
        {/* Musician thumbnail - circular with aspect-square to prevent CLS */}
        <div className="relative mx-auto mb-3 aspect-square w-full max-w-32 overflow-hidden rounded-full bg-slate-800">
          {thumbUrl ? (
            <img
              src={thumbUrl}
              alt={name}
              className="size-full object-cover transition-transform duration-300 group-hover:scale-105"
            />
          ) : (
            <div className="flex size-full items-center justify-center">
              <User className="size-10 text-slate-600" aria-hidden="true" />
            </div>
          )}
        </div>

        {/* Musician info */}
        <div className="text-center">
          <h3 className="truncate text-sm font-semibold text-white">{name}</h3>
          <p className="mt-0.5 text-xs text-slate-400">
            {album_count} {album_count === 1 ? "album" : "albums"} ·{" "}
            {track_count} {track_count === 1 ? "track" : "tracks"}
          </p>
        </div>
      </Link>
    </article>
  );
}
