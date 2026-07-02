import { fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import InTheatersCard from "@/components/InTheatersCard";
import MovieCard from "@/components/MovieCard";
import type { LatestMovieType, TheaterMovieType } from "@/types";
import { renderWithQueryClient } from "@/test/render";

vi.mock("@tanstack/react-router", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>(
      "@tanstack/react-router",
    );

  return {
    ...actual,
    Link: ({
      children,
      params,
      to,
      ...props
    }: {
      children: React.ReactNode;
      params?: { id?: string };
      to?: string;
    }) => {
      const href =
        typeof to === "string" ? to.replace("$id", params?.id ?? "") : "#";

      return (
        <a href={href} {...props}>
          {children}
        </a>
      );
    },
  };
});

const libraryMovie: LatestMovieType = {
  id: 1,
  title: "Fargo",
  poster_path: { Valid: true, String: "/fargo.jpg" },
  year: { Valid: true, Int64: 1996 },
};

const theaterMovie: TheaterMovieType = {
  id: 2,
  title: "Heat",
  original_title: "Heat",
  overview: "",
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

describe("poster load-error fallback", () => {
  it("MovieCard swaps a broken poster for the Film placeholder", () => {
    const { container } = renderWithQueryClient(
      <MovieCard movie={libraryMovie} />,
    );

    const img = container.querySelector("img");
    expect(img).not.toBeNull();

    fireEvent.error(img!);

    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("svg.lucide-film")).not.toBeNull();
  });

  it("InTheatersCard swaps a broken poster for the Film placeholder", () => {
    const { container } = renderWithQueryClient(
      <InTheatersCard movie={theaterMovie} />,
    );

    const img = container.querySelector("img");
    expect(img).not.toBeNull();

    fireEvent.error(img!);

    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("svg.lucide-film")).not.toBeNull();
  });
});
