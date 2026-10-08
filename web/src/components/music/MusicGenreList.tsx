import { Badge } from "@/components/ui/badge";

/** The genre pills under an album or musician title; nothing when empty. */
export default function MusicGenreList({ genres }: { genres: string[] }) {
  if (genres.length === 0) return null;

  return (
    <ul
      className="mt-4 flex list-none flex-wrap justify-center gap-2 lg:justify-start"
      aria-label={`Genres: ${genres.join(", ")}`}
    >
      {genres.map(genre => (
        <li key={genre}>
          <Badge
            variant="outline"
            className="border-primary/30 bg-muted/80 px-3 py-1 text-sm font-normal text-primary backdrop-blur-sm"
          >
            {genre}
          </Badge>
        </li>
      ))}
    </ul>
  );
}
