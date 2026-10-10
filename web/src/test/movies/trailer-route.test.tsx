import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { jsonResponse, requestURL } from "../helpers/api";
import { authUser } from "../helpers/fixtures";
import { renderRoute } from "../helpers/render-route";

vi.mock("@/hooks/useAudioPlayerActions", () => import("../helpers/audio-player"));

function mockTrailerFetch(movieResponse: () => Promise<Response>) {
  vi.stubGlobal(
    "fetch",
    vi.fn((input: RequestInfo | URL) => {
      const url = requestURL(input);

      if (url === "/api/auth/user") {
        return jsonResponse(authUser());
      }

      if (url.startsWith("/api/tmdb/movies/")) {
        return movieResponse();
      }

      return jsonResponse({ error: false, data: {} });
    }),
  );
}

describe("trailer route load errors", () => {
  // The server answers a lowercase constant; the UI owns the copy (§3.4).
  it("words a failed TMDB lookup for the reader instead of echoing the server", async () => {
    mockTrailerFetch(() =>
      jsonResponse({ error: true, message: "failed to fetch movie from tmdb" }, 500),
    );

    await renderRoute("/trailer?mediaType=movie&tmdbId=19");

    expect(
      await screen.findByRole("heading", { name: "Unable to Load Trailer" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Couldn’t load the trailer details from TMDB.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/failed to fetch movie from tmdb/)).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Try Again" })).toBeInTheDocument();
  });

  it("says the movie is missing on a 404", async () => {
    mockTrailerFetch(() =>
      jsonResponse({ error: true, message: "movie not found" }, 404),
    );

    await renderRoute("/trailer?mediaType=movie&tmdbId=19");

    expect(
      await screen.findByText("We couldn’t find that movie on TMDB."),
    ).toBeInTheDocument();
  });
});

describe("trailer route without a trailer", () => {
  it("lays the Go Back icon and label out on one line", async () => {
    mockTrailerFetch(() =>
      jsonResponse({
        error: false,
        data: { movie: { title: "Resident Evil", videos: { results: [] } } },
      }),
    );

    await renderRoute("/trailer?mediaType=movie&tmdbId=19");

    expect(
      await screen.findByRole("heading", { name: "No Trailer Available" }),
    ).toBeInTheDocument();
    // Without inline-flex the block-level svg stacks above the label.
    expect(screen.getByRole("button", { name: "Go Back" })).toHaveClass(
      "inline-flex",
      "items-center",
    );
  });
});
