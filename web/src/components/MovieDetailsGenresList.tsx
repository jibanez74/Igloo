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
          className="rounded-full border border-primary/30 bg-muted/80 px-3 py-1 text-sm text-primary backdrop-blur-sm"
        >
          {genre.tag}
        </li>
      ))}
    </ul>
  );
}
