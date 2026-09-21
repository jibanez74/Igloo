import { queryOptions } from "@tanstack/react-query";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Disc3, Film, Music, User } from "lucide-react";
import { describe, expect, it, vi } from "vitest";
import LibraryStats from "@/components/shared/LibraryStats";
import type { ApiResponseType } from "@/types";
import { renderWithQueryClient } from "../helpers/render";

type MoviesPayload = { total_movies: number };
type MusicPayload = {
  total_albums: number;
  total_tracks: number;
  total_musicians: number;
};

const MOVIE_NOUN = { singular: "movie", plural: "movies" };
const ALBUM_NOUN = { singular: "album", plural: "albums" };
const TRACK_NOUN = { singular: "track", plural: "tracks" };
const MUSICIAN_NOUN = { singular: "musician", plural: "musicians" };

function renderMovieStats(
  queryFn: () => Promise<ApiResponseType<MoviesPayload>>,
) {
  return renderWithQueryClient(
    <LibraryStats
      queryOpts={queryOptions({ queryKey: ["library-stats-movies"], queryFn })}
      figures={[
        {
          icon: Film,
          label: "Movies",
          noun: MOVIE_NOUN,
          getValue: data => data.total_movies,
        },
      ]}
    />,
  );
}

describe("LibraryStats", () => {
  it("names the region after one figure", async () => {
    renderMovieStats(async () => ({
      error: false,
      data: { total_movies: 42 },
    }));

    expect(
      await screen.findByRole("region", {
        name: "Library statistics: 42 movies",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("42")).toBeInTheDocument();
    expect(screen.getByText("Movies")).toBeInTheDocument();
  });

  it("joins several figures into the one accessible name", async () => {
    renderWithQueryClient(
      <LibraryStats
        queryOpts={queryOptions({
          queryKey: ["library-stats-music"],
          queryFn: async (): Promise<ApiResponseType<MusicPayload>> => ({
            error: false,
            data: { total_albums: 1, total_tracks: 5, total_musicians: 1 },
          }),
        })}
        figures={[
          {
            icon: Disc3,
            label: "Albums",
            noun: ALBUM_NOUN,
            getValue: data => data.total_albums,
          },
          {
            icon: Music,
            label: "Tracks",
            noun: TRACK_NOUN,
            getValue: data => data.total_tracks,
          },
          {
            icon: User,
            label: "Musicians",
            noun: MUSICIAN_NOUN,
            getValue: data => data.total_musicians,
          },
        ]}
      />,
    );

    expect(
      await screen.findByRole("region", {
        name: "Library statistics: 1 albums, 5 tracks, 1 musicians",
      }),
    ).toBeInTheDocument();
  });

  it("says it is loading until the counts arrive", () => {
    renderMovieStats(() => new Promise(() => {}));

    expect(
      screen.getByRole("region", { name: "Library statistics: loading" }),
    ).toBeInTheDocument();
    expect(screen.getByText("—")).toBeInTheDocument();
  });

  it("renders the API error envelope as an alert and refetches on retry", async () => {
    const user = userEvent.setup();
    const queryFn = vi
      .fn<() => Promise<ApiResponseType<MoviesPayload>>>()
      .mockResolvedValueOnce({ error: true, message: "Stats are unavailable." })
      .mockResolvedValueOnce({ error: false, data: { total_movies: 7 } });

    renderMovieStats(queryFn);

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Stats are unavailable.");

    await user.click(screen.getByRole("button", { name: "Try again" }));

    await waitFor(() => {
      expect(
        screen.getByRole("region", { name: "Library statistics: 7 movies" }),
      ).toBeInTheDocument();
    });
    expect(queryFn).toHaveBeenCalledTimes(2);
  });
});
