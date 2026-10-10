import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import InTheatersCard from "@/components/home/InTheatersCard";
import type { TheaterMovieType } from "@/types";
import { renderWithQueryClient } from "../helpers/render";

vi.mock("@tanstack/react-router", async () =>
  (await import("../helpers/router-link-mock")).routerWithAnchorLinks(),
);

const heat: TheaterMovieType = {
  id: 949,
  title: "Heat",
  original_title: "Heat",
  overview: "A crew and a detective.",
  release_date: "1995-12-15",
  poster_path: "/heat.jpg",
  backdrop_path: "",
  popularity: 10,
  vote_average: 8.3,
  vote_count: 100,
  adult: false,
  original_language: "en",
  genre_ids: [],
  video: false,
};

describe("InTheatersCard", () => {
  it("wears the TMDB score as a quiet corner badge the link already speaks", () => {
    renderWithQueryClient(<InTheatersCard movie={heat} />);

    expect(
      screen.getByRole("link", { name: "Heat, 1995, TMDB user score 8.3 out of 10" }),
    ).toBeInTheDocument();

    const badge = screen.getByText("8.3").closest('[data-slot="badge"]');
    expect(badge).toHaveAttribute("aria-hidden", "true");
    expect(badge).toHaveClass("absolute", "text-xs");
    expect(badge).toHaveTextContent("TMDB");
    expect(document.querySelector(".bg-aurora")).toBeNull();
    expect(document.querySelector("svg.lucide-star")).toBeNull();
  });
});
