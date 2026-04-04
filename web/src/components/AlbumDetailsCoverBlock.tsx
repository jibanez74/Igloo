import { Disc3 } from "lucide-react";

type AlbumDetailsCoverBlockProps = {
  coverUrl: string | null;
  albumTitle: string;
};

export default function AlbumDetailsCoverBlock({
  coverUrl,
  albumTitle,
}: AlbumDetailsCoverBlockProps) {
  return (
    <figure className="mx-auto min-w-0 shrink-0 lg:mx-0 lg:pt-1">
      <div className="w-44 overflow-hidden rounded-xl border border-amber-500/20 shadow-2xl shadow-amber-500/10 sm:w-52 md:w-64 lg:w-72">
        {coverUrl ? (
          <img
            src={coverUrl}
            alt={`Album cover for ${albumTitle}`}
            className="aspect-square w-full object-cover"
          />
        ) : (
          <div
            className="flex aspect-square w-full items-center justify-center bg-slate-800"
            role="img"
            aria-label="No cover available"
          >
            <Disc3 className="size-12 text-slate-600" aria-hidden="true" />
          </div>
        )}
      </div>
    </figure>
  );
}
