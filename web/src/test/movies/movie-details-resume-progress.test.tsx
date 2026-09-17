import { QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import MovieDetailsResumeProgress from "@/components/movies/MovieDetailsResumeProgress";
import { movieMediaRef } from "@/lib/media-ref";
import { mediaWatchProgressQueryKey } from "@/lib/query-opts";
import type { WatchProgressType } from "@/types";
import { jsonResponse, requestURL } from "../helpers/api";
import { createTestQueryClient } from "../helpers/render";

const MOVIE_ID = 7;

// 1890 of 7560 seconds watched: a quarter in, with 5670 seconds — an hour and
// thirty-five minutes — still to go.
const IN_PROGRESS: WatchProgressType = {
  progress_sec: 1890,
  duration_sec: 7560,
  watched: false,
  updated_at: "2026-07-16T12:00:00Z",
};

function renderStrip(progress: WatchProgressType) {
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = requestURL(input);
    if (url === `/api/movies/${MOVIE_ID}/watch-progress`) {
      return jsonResponse({ error: false, data: progress });
    }
    throw new Error(`unexpected request: ${url}`);
  });
  vi.stubGlobal("fetch", fetchMock);

  const queryClient = createTestQueryClient();
  render(
    <QueryClientProvider client={queryClient}>
      <MovieDetailsResumeProgress movieId={MOVIE_ID} />
    </QueryClientProvider>,
  );

  // The strip renders nothing at all for an ineligible position, so the
  // "renders nothing" cases have no element to wait on — they wait on the
  // query settling instead, or they would pass while it was still in flight.
  return vi.waitFor(() =>
    expect(
      queryClient.getQueryState(mediaWatchProgressQueryKey(movieMediaRef(MOVIE_ID)))
        ?.status,
    ).toBe("success"),
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("MovieDetailsResumeProgress", () => {
  it("shows the remaining time in hours and minutes", async () => {
    renderStrip(IN_PROGRESS);

    expect(await screen.findByText("1 hr 35 min left")).toBeInTheDocument();
  });

  // The abbreviated text is aria-hidden because screen readers read "hr" and
  // "min" inconsistently; the sr-only twin carries the spoken words.
  it("speaks the remaining time in full words", async () => {
    renderStrip(IN_PROGRESS);

    const spoken = await screen.findByText("1 hour 35 minutes left");
    expect(spoken).toHaveClass("sr-only");
    expect(screen.getByText("1 hr 35 min left")).toHaveAttribute(
      "aria-hidden",
      "true",
    );
  });

  it("renders nothing once the movie is marked watched", async () => {
    await renderStrip({ ...IN_PROGRESS, watched: true });

    expect(screen.queryByText(/left$/)).not.toBeInTheDocument();
  });

  it("renders nothing for progress too short to resume", async () => {
    await renderStrip({ ...IN_PROGRESS, progress_sec: 5 });

    expect(screen.queryByText(/left$/)).not.toBeInTheDocument();
  });
});
