import { useState } from "react";
import { Link } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Music, Play } from "lucide-react";
import { albumDetailsQueryOpts } from "@/lib/query-opts";
import { useAudioPlayer } from "@/hooks/useAudioPlayer";
import { Spinner } from "@/components/ui/spinner";
import type { SimpleAlbumType } from "@/types";

type AlbumCardProps = {
  album: SimpleAlbumType;
};

export default function AlbumCard({ album }: AlbumCardProps) {
  const { id, title, cover, musician } = album;
  const queryClient = useQueryClient();
  const audioPlayer = useAudioPlayer();
  const [isLoading, setIsLoading] = useState(false);

  const handlePrefetch = () =>
    queryClient.prefetchQuery(albumDetailsQueryOpts(id));

  const handlePlayAlbum = async (e: React.MouseEvent<HTMLButtonElement>) => {
    e.preventDefault();
    e.stopPropagation();
    setIsLoading(true);

    try {
      const data = await queryClient.fetchQuery(albumDetailsQueryOpts(id));

      if (!data.error && data.data?.tracks?.length > 0) {
        audioPlayer.playAlbum(data.data.tracks, {
          cover: data.data.album.cover?.Valid
            ? data.data.album.cover.String
            : null,
          title: data.data.album.title,
          musician: data.data.album.musician?.Valid
            ? data.data.album.musician.String
            : null,
        });
      }
    } catch (error) {
      console.error("Failed to load album:", error);

      toast.error("Unable to play album", {
        description: "Something went wrong. Please try again.",
      });
    } finally {
      setIsLoading(false);
    }
  };

  const coverUrl = cover.Valid ? cover.String : null;
  const musicianName = musician.Valid ? musician.String : null;

  return (
    <article
      className="group relative animate-in overflow-hidden rounded-xl border border-slate-800 bg-slate-900 transition-all duration-300 fade-in hover:-translate-y-1 hover:border-amber-400/50 hover:shadow-xl hover:shadow-amber-400/20"
      onMouseEnter={handlePrefetch}
      onFocus={handlePrefetch}
    >
      <Link
        to="/music/album/$id"
        params={{ id: id.toString() }}
        className="block focus:ring-2 focus:ring-amber-400 focus:outline-none focus:ring-inset"
        aria-label={`${title}${musicianName ? ` by ${musicianName}` : ""}`}
      >
        {/* Album cover */}
        <div className="relative aspect-square bg-slate-800">
          {coverUrl ? (
            <img
              src={coverUrl}
              alt=""
              width="640"
              height="640"
              loading="lazy"
              decoding="async"
              fetchPriority="low"
              sizes="(min-width: 1024px) 16.66vw, (min-width: 768px) 25vw, (min-width: 640px) 33.33vw, 50vw"
              className="size-full object-cover transition-transform duration-300 group-hover:scale-105"
            />
          ) : (
            <div className="flex size-full items-center justify-center">
              <Music className="size-10 text-slate-600" aria-hidden="true" />
            </div>
          )}

          {/* Play button overlay - appears on hover/focus */}
          <div
            className="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 transition-opacity duration-200 group-focus-within:opacity-100 group-hover:opacity-100"
            aria-hidden="true"
          />
        </div>

        {/* Album info */}
        <div className="p-3">
          <h3 className="truncate text-sm font-semibold text-white">{title}</h3>
          {musicianName && (
            <p className="mt-0.5 truncate text-xs text-slate-400">
              {musicianName}
            </p>
          )}
        </div>
      </Link>

      {/* Play button - positioned over the cover image */}
      <button
        onClick={handlePlayAlbum}
        disabled={isLoading}
        className="absolute top-1/2 left-1/2 flex size-12 -translate-x-1/2 -translate-y-[calc(50%+1rem)] scale-90 items-center justify-center rounded-full bg-amber-500 text-slate-900 opacity-0 shadow-lg shadow-black/30 transition-all duration-200 group-focus-within:scale-100 group-focus-within:opacity-100 group-hover:scale-100 group-hover:opacity-100 hover:bg-amber-400 focus:scale-100 focus:opacity-100 focus:ring-2 focus:ring-white focus:ring-offset-2 focus:ring-offset-slate-900 focus:outline-none disabled:opacity-50"
        aria-label={`Play ${title}${musicianName ? ` by ${musicianName}` : ""}`}
      >
        {isLoading ? (
          <Spinner className="size-5" />
        ) : (
          <Play className="size-5 fill-current" aria-hidden="true" />
        )}
      </button>
    </article>
  );
}
