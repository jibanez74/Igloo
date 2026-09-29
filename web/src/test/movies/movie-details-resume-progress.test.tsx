import { screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import MovieDetailsResumeProgress from "@/components/movies/MovieDetailsResumeProgress";
import { movieMediaRef } from "@/lib/media-ref";
import { mediaWatchProgressQueryKey } from "@/lib/query-opts";
import type { WatchProgressType } from "@/types";
import { jsonResponse, requestURL } from "../helpers/api";
import { createTestQueryClient, renderWithQueryClient } from "../helpers/render";

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
  renderWithQueryClient(
    <MovieDetailsResumeProgress movieId={MOVIE_ID} />,
    { queryClient },
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

  it("fills the bar to the share already watched", async () => {
    renderStrip(IN_PROGRESS);

    const caption = (await screen.findByText("1 hr 35 min left")).closest("p");
    const track = caption?.previousElementSibling;
    expect(track).toHaveAttribute("aria-hidden", "true");
    expect((track?.firstElementChild as HTMLElement).style.width).toBe("25%");
  });

  it("renders nothing once the movie is marked watched", async () => {
    await renderStrip({ ...IN_PROGRESS, watched: true });

    expect(screen.queryByText(/left$/)).not.toBeInTheDocument();
  });

  it("renders nothing for progress too short to resume", async () => {
    await renderStrip({ ...IN_PROGRESS, progress_sec: 5 });

    expect(screen.queryByText(/left$/)).not.toBeInTheDocument();
  });

  // Past the completion threshold the movie counts as finished even before
  // the watched flag is saved, so there is nothing left to resume.
  it("renders nothing once the position passes the completion threshold", async () => {
    await renderStrip({ ...IN_PROGRESS, progress_sec: 980, duration_sec: 1000 });

    expect(screen.queryByText(/left$/)).not.toBeInTheDocument();
  });
});
