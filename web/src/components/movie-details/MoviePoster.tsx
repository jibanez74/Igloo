import { Film } from "lucide-react";

type MoviePosterProps = {
  src: string | null;
  title: string;
  className?: string;
};

export default function MoviePoster({
  src,
  title,
  className = "",
}: MoviePosterProps) {
  return (
    <figure
      className={`mx-auto shrink-0 md:mx-0 ${className}`.trim()}
    >
      <div className="w-48 overflow-hidden rounded-xl border border-amber-500/20 shadow-2xl shadow-amber-500/10 md:w-64 lg:w-72">
        {src ? (
          <img
            src={src}
            alt={`Movie poster for ${title}`}
            className="aspect-2/3 w-full object-cover"
          />
        ) : (
          <div
            className="flex aspect-2/3 w-full items-center justify-center bg-slate-800"
            role="img"
            aria-label="No poster available"
          >
            <Film className="size-12 text-slate-600" aria-hidden="true" />
          </div>
        )}
      </div>
    </figure>
  );
}
