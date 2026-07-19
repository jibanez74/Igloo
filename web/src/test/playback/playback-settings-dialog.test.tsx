import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import PlaybackSettingsDialog from "@/components/movies/PlaybackSettingsDialog";
import {
  MOTION_MEDIA_DIALOG_SURFACE_CLASS,
  MOVIE_TECHNICAL_DETAILS_KEY,
} from "@/lib/constants";
import type {
  ApiResponseType,
  AudioStreamType,
  MovieTechnicalDetailsResponse,
  SubtitleType,
  VideoStreamType,
} from "@/types";

function nullableString(value = "") {
  return {
    String: value,
    Valid: value.length > 0,
  };
}

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

function videoStream(): VideoStreamType {
  return {
    id: 1,
    movie_id: 22,
    stream_index: 0,
    codec: "h264",
    codec_profile: nullableString("Main"),
    codec_level: nullableInt64(41),
    bit_rate: 4_000_000,
    width: 1920,
    height: 1080,
    coded_width: nullableInt64(1920),
    coded_height: nullableInt64(1080),
    aspect_ratio: nullableString("16:9"),
    frame_rate: 24,
    avg_frame_rate: nullableString("24/1"),
    bit_depth: nullableInt64(8),
    color_range: nullableString(),
    color_space: nullableString(),
    color_primaries: nullableString(),
    color_transfer: nullableString(),
    language: nullableString(),
    title: nullableString(),
  };
}

function audioStream(
  overrides: Partial<AudioStreamType> = {},
): AudioStreamType {
  return {
    id: 1,
    movie_id: 22,
    stream_index: 1,
    codec: "aac",
    codec_profile: nullableString("LC"),
    bit_rate: 192_000,
    sample_rate: nullableInt64(48_000),
    channels: 2,
    channel_layout: nullableString("stereo"),
    language: nullableString("eng"),
    title: nullableString("English Stereo"),
    ...overrides,
  };
}

function subtitleStream(): SubtitleType {
  return {
    id: 1,
    movie_id: 22,
    stream_index: 3,
    codec: "subrip",
    language: nullableString("eng"),
    title: nullableString("SDH"),
    is_forced: false,
    is_default: true,
  };
}

function technicalDetails(): ApiResponseType<MovieTechnicalDetailsResponse> {
  return {
    error: false,
    data: {
      movie: {
        file_name: "arrival.mp4",
        file_path: "/media/arrival.mp4",
        size: 1000,
        container: "mp4",
        mime_type: "video/mp4",
        run_time: nullableInt64(116),
        duration: nullableFloat64(6960),
      },
      video_streams: [videoStream()],
      audio_streams: [
        audioStream(),
        audioStream({
          id: 2,
          stream_index: 2,
          language: nullableString("spa"),
          title: nullableString("Spanish Stereo"),
        }),
      ],
      subtitles: [subtitleStream()],
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

describe("PlaybackSettingsDialog", () => {
  it("renders labelled controls inside the media dialog surface", () => {
    const queryClient = createQueryClient();
    queryClient.setQueryData([MOVIE_TECHNICAL_DETAILS_KEY, 22], technicalDetails());

    render(
      <QueryClientProvider client={queryClient}>
        <PlaybackSettingsDialog
          movieId={22}
          open
          onOpenChange={vi.fn()}
          settings={{ mode: "direct", audioTrack: 0, subtitleTrack: null }}
          onSave={vi.fn()}
        />
      </QueryClientProvider>,
    );

    expect(screen.getByRole("dialog")).toHaveClass(
      ...MOTION_MEDIA_DIALOG_SURFACE_CLASS.split(" "),
    );
    expect(
      screen.getByRole("heading", { name: "Playback Settings" }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Playback")).toBeInTheDocument();
    expect(screen.getByLabelText("Audio Track")).toBeInTheDocument();
    expect(screen.getByLabelText("Subtitles")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Done" })).toBeInTheDocument();
  });
});
