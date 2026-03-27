import type { MovieDetailsBackdropProps } from "@/types";

export default function MovieDetailsBackdrop({
  backdropUrl,
}: MovieDetailsBackdropProps) {
  return (
    <header className="relative -mx-4 sm:-mx-6 lg:-mx-8">
      {backdropUrl ? (
        <img
          src={backdropUrl}
          alt=""
          aria-hidden="true"
          className="h-44 w-full object-cover object-top sm:h-52 md:aspect-21/9 md:h-auto md:max-h-[min(42vh,22rem)] md:min-h-48"
        />
      ) : (
        <div
          className="h-44 w-full bg-slate-800 sm:h-52 md:aspect-21/9 md:min-h-48"
          aria-hidden="true"
        />
      )}
      <div
        className="absolute inset-0 bg-linear-to-t from-slate-950 via-slate-950/60 to-transparent"
        aria-hidden="true"
      />
    </header>
  );
}
