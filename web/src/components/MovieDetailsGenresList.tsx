import type { MovieDetailsGenresListProps } from "@/types";

export default function MovieDetailsGenresList({
  genres,
}: MovieDetailsGenresListProps) {
  if (genres.length === 0) return null;

  return (
    <ul
      className="mt-4 flex list-none flex-wrap justify-center gap-2 lg:justify-start"
      aria-label={`Genres: ${genres.map(g => g.tag).join(", ")}`}
    >
      {genres.map(genre => (
        <li
          key={genre.id}
          className="rounded-full border border-amber-500/30 bg-slate-800/80 px-3 py-1 text-sm text-amber-200 backdrop-blur-sm"
        >
          {genre.tag}
        </li>
      ))}
    </ul>
  );
}
