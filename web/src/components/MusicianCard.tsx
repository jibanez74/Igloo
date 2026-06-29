import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { User } from "lucide-react";
import {
  CARD_INTERACTIVE_SURFACE_CLASS,
  CARD_MEDIA_HOVER_CLASS,
} from "@/lib/constants";
import { unwrapString } from "@/lib/nullable";
import { getMediaImageUrl } from "@/lib/media-image-url";
import { cn } from "@/lib/utils";
import type { SimpleMusicianType } from "@/types";

type MusicianCardProps = {
  musician: SimpleMusicianType;
};

export default function MusicianCard({ musician }: MusicianCardProps) {
  const { id, name, thumb, album_count, track_count } = musician;
  const thumbUrl = getMediaImageUrl(unwrapString(thumb));
  const [failedThumbUrl, setFailedThumbUrl] = useState<string | null>(null);

  const showThumb = thumbUrl && thumbUrl !== failedThumbUrl;

  return (
    <article
      className={cn(
        CARD_INTERACTIVE_SURFACE_CLASS,
        "group relative overflow-hidden rounded-xl border border-slate-800 bg-slate-900 p-4 hover:-translate-y-1 hover:border-amber-400/50 hover:shadow-xl hover:shadow-amber-400/20",
      )}
    >
      <Link
        to="/music/musician/$id"
        params={{ id: id.toString() }}
        className="block focus:ring-2 focus:ring-ring focus:outline-none focus:ring-inset"
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
              className={cn("size-full object-cover", CARD_MEDIA_HOVER_CLASS)}
              onError={() => setFailedThumbUrl(thumbUrl)}
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
