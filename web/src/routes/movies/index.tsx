import { createFileRoute } from "@tanstack/react-router";
import { z } from "zod";

const searchSchema = z.object({
  currentPage?: number;
  totalPages?: number;
});

export const Route = createFileRoute("/movies/")({
  component: MoviesPage,
  validateSearch: searchSchema.parse,
});

function MoviesPage() {
  return (
    <div>
      <h1>movies go here</h1>
    </div>
  );
}
