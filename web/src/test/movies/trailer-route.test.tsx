import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { jsonResponse, requestURL } from "../helpers/api";
import { authUser } from "../helpers/fixtures";
import { renderRoute } from "../helpers/render-route";

vi.mock("@/hooks/useAudioPlayerActions", () => ({
  useAudioPlayerActions: () => ({
    pause: vi.fn(),
    suspendKeyboard: vi.fn(),
    resumeKeyboard: vi.fn(),
  }),
}));

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
        "Couldn’t load the trailer details from TMDB. Check your connection and try again.",
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
