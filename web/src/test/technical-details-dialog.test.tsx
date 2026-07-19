import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import TechnicalDetailsDialog from "@/components/movies/TechnicalDetailsDialog";
import { MOVIE_TECHNICAL_DETAILS_KEY } from "@/lib/constants";
import type {
  ApiResponseType,
  MovieTechnicalDetailsResponse,
} from "@/types";

function nullableInt64(value: number | null = null) {
  return {
    Int64: value ?? 0,
    Valid: value != null,
  };
}

function nullableFloat64(value: number | null = null) {
  return {
    Float64: value ?? 0,
    Valid: value != null,
  };
}

function technicalDetails(
  runtimeMinutes: number | null,
): ApiResponseType<MovieTechnicalDetailsResponse> {
  return {
    error: false,
    data: {
      movie: {
        file_name: "arrival.mp4",
        file_path: "/media/arrival.mp4",
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

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  });
}

function renderDialog(runtimeMinutes: number | null) {
  const queryClient = createQueryClient();
  queryClient.setQueryData(
    [MOVIE_TECHNICAL_DETAILS_KEY, 22],
    technicalDetails(runtimeMinutes),
  );

  render(
    <QueryClientProvider client={queryClient}>
      <TechnicalDetailsDialog
        movieId={22}
        open
        onOpenChange={vi.fn()}
      />
    </QueryClientProvider>,
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

  it("renders rounded duration when runtime is displayable", () => {
    renderDialog(116);

    expect(
      screen.getByText("Duration (rounded, for display)"),
    ).toBeInTheDocument();
    expect(screen.getByText("1 hr 56 min")).toBeInTheDocument();
  });
});
