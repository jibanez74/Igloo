import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { z } from "zod";

const searchSchema = z.object({
  currentPage: z.number().optional(),
  totalPages: z.number().optional(),
});

export const Route = createFileRoute("/movies/")({
  component: MoviesPage,
  validateSearch: searchSchema.parse,
});

function MoviesPage() {
  const navigate = useNavigate();

  const goToMovie = (movieId: string) => {
    navigate({ to: '/movies/$movieId', params: { movieId } });
  };

  return (
    <div>
      <h1>movies go here</h1>
      <button onClick={() => goToMovie('123')}>Go to Movie 123</button>
    </div>
  );
}
