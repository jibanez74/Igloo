import type { MovieGenreView } from "@/lib/movie-details-view";

type MovieGenresProps = {
  genres: MovieGenreView[];
};

export default function MovieGenres({ genres }: MovieGenresProps) {
  if (!genres.length) return null;

  const label = `Genres: ${genres.map(g => g.name).join(", ")}`;

  return (
    <ul
      className="mt-4 flex list-none flex-wrap gap-2"
      aria-label={label}
    >
      {genres.map((genre, index) => (
        <li
          key={genre.id}
          tabIndex={0}
          role="listitem"
          aria-posinset={index + 1}
          aria-setsize={genres.length}
          className="rounded-full border border-amber-500/30 bg-slate-800/80 px-3 py-1 text-sm text-amber-200 backdrop-blur-sm outline-none focus-visible:border-amber-400 focus-visible:ring-2 focus-visible:ring-amber-400/50"
        >
          {genre.name}
        </li>
      ))}
    </ul>
  );
}
