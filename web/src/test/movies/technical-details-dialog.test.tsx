import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import TechnicalDetailsDialog from "@/components/movies/TechnicalDetailsDialog";
import {
  DIALOG_SCROLL_BODY_CLASS,
  DIALOG_SCROLL_CONTENT_CLASS,
  MOVIE_TECHNICAL_DETAILS_KEY,
} from "@/lib/constants";
import type {
  ApiResponseType,
  MovieTechnicalDetailsResponse,
} from "@/types";
import { nullableFloat64, nullableInt64 } from "../helpers/fixtures";
import { createTestQueryClient, renderWithQueryClient } from "../helpers/render";

function technicalDetails(
  runtimeMinutes: number | null,
): ApiResponseType<MovieTechnicalDetailsResponse> {
  return {
    error: false,
    data: {
      movie: {
        file_name: "arrival.mp4",
        size: 1000,
        container: "mp4",
        mime_type: "video/mp4",
        run_time: nullableInt64(runtimeMinutes),
        duration: nullableFloat64(6960),
      },
      video_streams: [],
      audio_streams: [],
      subtitles: [],
      chapters: [],
    },
  };
}

function renderDialog(runtimeMinutes: number | null) {
  const queryClient = createTestQueryClient();
  queryClient.setQueryData(
    [MOVIE_TECHNICAL_DETAILS_KEY, 22],
    technicalDetails(runtimeMinutes),
  );

  renderWithQueryClient(
    <TechnicalDetailsDialog
      movieId={22}
      open
      onOpenChange={vi.fn()}
    />,
    { queryClient },
  );
}

describe("TechnicalDetailsDialog", () => {
  it("does not render rounded duration when runtime formats to no value", () => {
    renderDialog(0);

    expect(
      screen.queryByText("Duration (rounded, for display)"),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Exact duration (ffprobe)")).toBeInTheDocument();
    expect(screen.getByText("6960.00 s")).toBeInTheDocument();
  });

  // Focus is moved to the title by the dialog, not by the reader, so it must
  // not ring (design-system §1.7); and the content box caps while only the
  // body scrolls, so the close button never sits on the scrollbar track.
  it("focuses the heading without a ring and scrolls only its body", async () => {
    renderDialog(116);

    const title = screen.getByRole("heading", { name: "Technical details" });
    await vi.waitFor(() => expect(title).toHaveFocus());
    expect(title).toHaveClass("outline-hidden");
    expect(title.className).not.toMatch(/ring-/);

    const content = screen.getByRole("dialog");
    for (const cls of DIALOG_SCROLL_CONTENT_CLASS.split(" ")) {
      expect(content).toHaveClass(cls);
    }
    const body = screen.getByText("Exact duration (ffprobe)").closest(
      `.${DIALOG_SCROLL_BODY_CLASS.split(" ").join(".")}`,
    );
    expect(body).not.toBeNull();
    expect(body).not.toContainElement(title);
    expect(body).not.toContainElement(screen.getByRole("button", { name: "Close" }));
  });

  it("renders rounded duration when runtime is displayable", () => {
    renderDialog(116);

    expect(
      screen.getByText("Duration (rounded, for display)"),
    ).toBeInTheDocument();
    expect(screen.getByText("1 hr 56 min")).toBeInTheDocument();
  });
});
