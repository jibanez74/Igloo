import { createFileRoute } from "@tanstack/react-router";
import type { SimpleMovie } from "@/types/Movie";

export const Route = createFileRoute("/movies/")({
  component: MoviesPage,
  loader: async (): Promise<SimpleMovie[]> => {
    const res = await fetch("/api/v1/auth/movies", {
      method: "get",
      credentials: "include",
      headers: {
        "Content-Type": "application/json",
      },
    });

    const r = await res.json();

    return r.movies;
  },
});

function MoviesPage() {
  return (
    <section>
      <div>movies go here</div>
    </section>
  );
}
