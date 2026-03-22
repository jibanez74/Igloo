import { useEffect, useState } from "react";
import { Link } from "@tanstack/react-router";
import { User } from "lucide-react";
import { unwrapString } from "@/lib/nullable";
import { getMediaImageUrl } from "@/lib/media-image-url";
import type { SimpleMusicianType } from "@/types";

type MusicianCardProps = {
  musician: SimpleMusicianType;
};

export default function MusicianCard({ musician }: MusicianCardProps) {
  const { id, name, thumb, album_count, track_count } = musician;
  const thumbUrl = getMediaImageUrl(unwrapString(thumb));
  const [thumbLoadFailed, setThumbLoadFailed] = useState(false);

  // Reset load-failed state when musician or thumb URL changes. Deferred to avoid
  // synchronous setState in effect (react-hooks/set-state-in-effect).
  useEffect(() => {
    queueMicrotask(() => setThumbLoadFailed(false));
  }, [id, thumbUrl]);

  const showThumb = thumbUrl && !thumbLoadFailed;

  return (
    <article className="group relative animate-in overflow-hidden rounded-xl border border-slate-800 bg-slate-900 p-4 transition-all duration-300 fade-in hover:-translate-y-1 hover:border-amber-400/50 hover:shadow-xl hover:shadow-amber-400/20">
      <Link
        to="/music/musician/$id"
        params={{ id: id.toString() }}
        className="block focus:ring-2 focus:ring-amber-400 focus:outline-none focus:ring-inset"
        aria-label={`${name}, ${album_count} albums, ${track_count} tracks`}
      >
        {/* Musician thumbnail - circular; fallback to User icon on load error */}
        <div className="relative mx-auto mb-3 aspect-square w-full max-w-32 overflow-hidden rounded-full bg-slate-800">
          {showThumb ? (
            <img
              src={thumbUrl}
              alt={`Photo of ${name}`}
              loading="lazy"
              decoding="async"
              className="size-full object-cover transition-transform duration-300 group-hover:scale-105"
              onError={() => setThumbLoadFailed(true)}
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
