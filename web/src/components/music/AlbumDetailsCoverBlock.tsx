import { Disc3 } from "lucide-react";
import { usePosterFallback } from "@/hooks/usePosterFallback";

type AlbumDetailsCoverBlockProps = {
  coverUrl: string | null;
  albumTitle: string;
};

export default function AlbumDetailsCoverBlock({
  coverUrl,
  albumTitle,
}: AlbumDetailsCoverBlockProps) {
  const cover = coverUrl ?? "";
  const { showPoster: showImage, onError } = usePosterFallback(cover);

  return (
    <figure className="mx-auto min-w-0 shrink-0 lg:mx-0 lg:pt-1">
      <div className="w-44 overflow-hidden rounded-xl border border-primary/20 shadow-2xl shadow-primary/10 sm:w-52 md:w-64 lg:w-72">
        {showImage ? (
          <img
            src={cover}
            alt={`Album cover for ${albumTitle}`}
            loading="lazy"
            decoding="async"
            className="aspect-square w-full object-cover"
            onError={onError}
          />
        ) : (
          <div
            className="flex aspect-square w-full items-center justify-center bg-muted"
            role="img"
            aria-label="No cover available"
          >
            <Disc3 className="size-12 text-muted-foreground" aria-hidden="true" />
          </div>
        )}
      </div>
    </figure>
  );
}
