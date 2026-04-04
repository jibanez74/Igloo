import { Disc3 } from "lucide-react";

type AlbumDetailsBackdropProps = {
  coverUrl: string | null;
  albumTitle: string;
};

export default function AlbumDetailsBackdrop({
  coverUrl,
  albumTitle,
}: AlbumDetailsBackdropProps) {
  return (
    <header className="relative -mx-4 sm:-mx-6 lg:-mx-8">
      {coverUrl ? (
        <img
          src={coverUrl}
          alt=""
          aria-hidden="true"
          className="h-44 w-full object-cover object-center sm:h-52 md:aspect-21/9 md:h-auto md:max-h-[min(42vh,22rem)] md:min-h-48"
        />
      ) : (
        <div
          className="flex h-44 w-full items-center justify-center bg-slate-800 sm:h-52 md:aspect-21/9 md:min-h-48"
          aria-hidden="true"
        >
          <Disc3 className="size-16 text-slate-600 opacity-40" aria-hidden="true" />
        </div>
      )}
      <div
        className="absolute inset-0 bg-linear-to-t from-slate-950 via-slate-950/60 to-transparent"
        aria-hidden="true"
      />
      <span className="sr-only">Album artwork for {albumTitle}</span>
    </header>
  );
}
