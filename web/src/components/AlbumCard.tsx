import { Link } from "@tanstack/react-router";
import type { SimpleAlbumType } from "@/types";

type AlbumCardProps = {
  album: SimpleAlbumType;
};

export default function AlbumCard({ album }: AlbumCardProps) {
  const { id, title, cover } = album;

  return (
    <div className="group rounded-xl overflow-hidden bg-slate-900 border border-slate-800 transition-all duration-300 hover:shadow-xl hover:shadow-amber-400/20 hover:border-amber-400/50 hover:-translate-y-1">
      <Link
        to={`/music/album/$id`}
        params={{
          id: id.toString(),
        }}
        className="block focus:outline-none focus:ring-2 focus:ring-amber-400"
      >
        <div className="aspect-square bg-slate-800">
          {cover ? (
            <img
              src={cover}
              alt=""
              width="640"
              height="640"
              loading="lazy"
              decoding="async"
              fetchPriority="low"
              sizes="(min-width: 1024px) 16.66vw, (min-width: 768px) 25vw, (min-width: 640px) 33.33vw, 50vw"
              className="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
            />
          ) : (
            <div className="h-full w-full flex items-center justify-center">
              <i className="fa-solid fa-music text-slate-600 text-4xl"></i>
            </div>
          )}
        </div>

        <div className="p-3">
          <h3 className="text-sm font-semibold truncate">{title}</h3>
        </div>
      </Link>
    </div>
  );
}
