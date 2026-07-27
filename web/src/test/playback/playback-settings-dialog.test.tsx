import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import PlaybackSettingsDialog from "@/components/movies/PlaybackSettingsDialog";
import {
  AUDIO_TRACK_MODE_NOTE,
  AUDIO_TRACK_MODE_NOTE_ID,
  MOTION_MEDIA_DIALOG_SURFACE_CLASS,
  MOVIE_TECHNICAL_DETAILS_KEY,
  PLAYBACK_SETTINGS_SUMMARY_LOADING,
} from "@/lib/constants";
import type {
  ApiResponseType,
  AudioStreamType,
  MovieTechnicalDetailsResponse,
  SubtitleType,
  VideoStreamType,
} from "@/types";

const prefersCoarse = vi.hoisted(() => ({ value: false }));
vi.mock("@/hooks/use-coarse-pointer", () => ({
  usePrefersCoarsePointer: () => prefersCoarse.value,
}));

afterEach(() => {
  prefersCoarse.value = false;
});

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
    is_default: false,
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

  it("saves the selected mode, audio track, and subtitle as one draft", () => {
    prefersCoarse.value = true;
    const queryClient = createQueryClient();
    queryClient.setQueryData(
      [MOVIE_TECHNICAL_DETAILS_KEY, 22],
      technicalDetails(),
    );
    const onSave = vi.fn();
    const onOpenChange = vi.fn();

    render(
      <QueryClientProvider client={queryClient}>
        <PlaybackSettingsDialog
          movieId={22}
          open
          onOpenChange={onOpenChange}
          settings={{ mode: "direct", audioTrack: 0, subtitleTrack: null }}
          onSave={onSave}
        />
      </QueryClientProvider>,
    );

    fireEvent.change(screen.getByLabelText("Playback"), {
      target: { value: "720p_3mbps" },
    });
    fireEvent.change(screen.getByLabelText("Audio Track"), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText("Subtitles"), {
      target: { value: "0" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Done" }));

    expect(onSave).toHaveBeenCalledWith({
      mode: "720p_3mbps",
      audioTrack: 1,
      subtitleTrack: 0,
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("disables image-based subtitle options", () => {
    prefersCoarse.value = true;
    const queryClient = createQueryClient();
    const details = technicalDetails();
    details.data!.subtitles.push({
      ...subtitleStream(),
      id: 2,
      stream_index: 4,
      codec: "hdmv_pgs_subtitle",
      title: nullableString("Signs"),
    });
    queryClient.setQueryData([MOVIE_TECHNICAL_DETAILS_KEY, 22], details);

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

    const bitmapOption = screen.getByRole("option", {
      name: /\(image-based\)/,
    });
    expect(bitmapOption).toBeDisabled();

    const textOption = screen.getByRole("option", { name: /SDH/ });
    expect(textOption).not.toBeDisabled();
  });

  it("offers no modes and disables saving while technical details load", () => {
    prefersCoarse.value = true;
    const queryClient = createQueryClient();

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

    const modeSelect = screen.getByLabelText("Playback");
    expect(modeSelect).toBeDisabled();
    expect(
      screen.queryByRole("option", { name: /Original file/ }),
    ).toBeNull();

    expect(screen.getByRole("button", { name: "Done" })).toBeDisabled();
    expect(
      screen.getAllByText(PLAYBACK_SETTINGS_SUMMARY_LOADING).length,
    ).toBeGreaterThan(0);
  });

  it("only offers modes the source supports once technical details arrive", () => {
    prefersCoarse.value = true;
    const queryClient = createQueryClient();
    const details = technicalDetails();
    details.data!.movie.mime_type = "video/x-matroska";
    details.data!.movie.container = "mkv";
    details.data!.video_streams[0].codec = "hevc";
    queryClient.setQueryData([MOVIE_TECHNICAL_DETAILS_KEY, 22], details);

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

    expect(
      screen.queryByRole("option", { name: /Original file/ }),
    ).toBeNull();
    expect(
      screen.queryByRole("option", { name: /Original video/ }),
    ).toBeNull();
    expect(
      screen.getByRole("option", { name: /1080p — best quality/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: /720p — lower bandwidth/ }),
    ).toBeInTheDocument();
  });

  it.each([
    ["native", true],
    ["Radix", false],
  ])(
    "keeps %s playback and audio controls disabled when the source has no mode",
    (_, coarsePointer) => {
      prefersCoarse.value = coarsePointer;
      const queryClient = createQueryClient();
      const details = technicalDetails();
      details.data!.movie.mime_type = "video/x-matroska";
      details.data!.movie.container = "mkv";
      details.data!.video_streams[0].codec = "hevc";
      details.data!.video_streams[0].height = 480;
      queryClient.setQueryData([MOVIE_TECHNICAL_DETAILS_KEY, 22], details);
      const onSave = vi.fn();

      render(
        <QueryClientProvider client={queryClient}>
          <PlaybackSettingsDialog
            movieId={22}
            open
            onOpenChange={vi.fn()}
            settings={{ mode: "direct", audioTrack: 0, subtitleTrack: null }}
            onSave={onSave}
          />
        </QueryClientProvider>,
      );

      const modeSelect = screen.getByLabelText("Playback");
      const audioSelect = screen.getByLabelText("Audio Track");
      const doneButton = screen.getByRole("button", { name: "Done" });

      expect(modeSelect).toBeDisabled();
      expect(audioSelect).toBeDisabled();
      expect(doneButton).toBeDisabled();

      if (coarsePointer) {
        fireEvent.change(audioSelect, { target: { value: "1" } });
      } else {
        fireEvent.click(audioSelect);
      }
      fireEvent.click(doneButton);

      expect(modeSelect).toBeDisabled();
      expect(audioSelect).toBeDisabled();
      expect(doneButton).toBeDisabled();
      expect(onSave).not.toHaveBeenCalled();
    },
  );

  it("switches direct play to remux when a non-first audio track is picked", () => {
    prefersCoarse.value = true;
    const queryClient = createQueryClient();
    queryClient.setQueryData(
      [MOVIE_TECHNICAL_DETAILS_KEY, 22],
      technicalDetails(),
    );
    const onSave = vi.fn();

    render(
      <QueryClientProvider client={queryClient}>
        <PlaybackSettingsDialog
          movieId={22}
          open
          onOpenChange={vi.fn()}
          settings={{ mode: "direct", audioTrack: 0, subtitleTrack: null }}
          onSave={onSave}
        />
      </QueryClientProvider>,
    );

    const modeSelect = screen.getByLabelText("Playback");
    const audioSelect = screen.getByLabelText("Audio Track");
    expect(modeSelect).toHaveValue("direct");
    expect(screen.queryByText(AUDIO_TRACK_MODE_NOTE)).toBeNull();
    expect(audioSelect).not.toHaveAttribute("aria-describedby");

    fireEvent.change(audioSelect, { target: { value: "1" } });

    expect(modeSelect).toHaveValue("remux");
    expect(screen.getByText(AUDIO_TRACK_MODE_NOTE)).toBeInTheDocument();
    expect(audioSelect).toHaveAttribute(
      "aria-describedby",
      AUDIO_TRACK_MODE_NOTE_ID,
    );
    expect(modeSelect).toHaveAttribute(
      "aria-describedby",
      AUDIO_TRACK_MODE_NOTE_ID,
    );

    fireEvent.click(screen.getByRole("button", { name: "Done" }));
    expect(onSave).toHaveBeenCalledWith({
      mode: "remux",
      audioTrack: 1,
      subtitleTrack: null,
    });
  });

  it("snaps the audio track back to the first stream when direct play is chosen", () => {
    prefersCoarse.value = true;
    const queryClient = createQueryClient();
    queryClient.setQueryData(
      [MOVIE_TECHNICAL_DETAILS_KEY, 22],
      technicalDetails(),
    );
    const onSave = vi.fn();

    render(
      <QueryClientProvider client={queryClient}>
        <PlaybackSettingsDialog
          movieId={22}
          open
          onOpenChange={vi.fn()}
          settings={{ mode: "remux", audioTrack: 1, subtitleTrack: null }}
          onSave={onSave}
        />
      </QueryClientProvider>,
    );

    const modeSelect = screen.getByLabelText("Playback");
    const audioSelect = screen.getByLabelText("Audio Track");
    expect(screen.getByText(AUDIO_TRACK_MODE_NOTE)).toBeInTheDocument();

    fireEvent.change(modeSelect, { target: { value: "direct" } });

    expect(audioSelect).toHaveValue("0");
    expect(screen.queryByText(AUDIO_TRACK_MODE_NOTE)).toBeNull();
    expect(modeSelect).not.toHaveAttribute("aria-describedby");

    fireEvent.click(screen.getByRole("button", { name: "Done" }));
    expect(onSave).toHaveBeenCalledWith({
      mode: "direct",
      audioTrack: 0,
      subtitleTrack: null,
    });
  });
});
