import type { LibraryMovieGenreType } from "@/types/movies";

type MovieDetailsGenresListProps = {
  genres: LibraryMovieGenreType[];
};

export default function MovieDetailsGenresList({
  genres,
}: MovieDetailsGenresListProps) {
  if (genres.length === 0) return null;

  return (
    <ul
      className="mt-3 flex list-none flex-wrap items-center justify-center gap-x-2 gap-y-1 text-sm text-white/75 drop-shadow-sm lg:justify-start"
      aria-label={`Genres: ${genres.map(g => g.tag).join(", ")}`}
    >
      {genres.map((genre, index) => (
        <li key={genre.id} className="flex items-center gap-x-2">
          {index > 0 && <span aria-hidden="true">·</span>}
          {genre.tag}
        </li>
      ))}
    </ul>
  );
}
