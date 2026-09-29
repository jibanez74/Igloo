import { fireEvent, screen } from "@testing-library/react";
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
  MovieTechnicalDetailsResponse,
  SubtitleType,
} from "@/types";
import { nullableFloat64, nullableInt64, nullableString } from "../helpers/fixtures";
import { createTestQueryClient, renderWithQueryClient } from "../helpers/render";
import {
  audioStream,
  subtitleStream,
  videoStream,
} from "../helpers/tech-details";

const prefersCoarse = vi.hoisted(() => ({ value: false }));
vi.mock("@/hooks/use-coarse-pointer", () => ({
  usePrefersCoarsePointer: () => prefersCoarse.value,
}));

afterEach(() => {
  prefersCoarse.value = false;
});

// The default English SDH subtitle this dialog's fixtures carry.
function sdhSubtitle(overrides: Partial<SubtitleType> = {}): SubtitleType {
  return subtitleStream({
    stream_index: 3,
    title: nullableString("SDH"),
    is_default: true,
    ...overrides,
  });
}

function technicalDetails(): ApiResponseType<MovieTechnicalDetailsResponse> {
  return {
    error: false,
    data: {
      movie: {
        file_name: "arrival.mp4",
        size: 1000,
        container: "mp4",
        mime_type: "video/mp4",
        run_time: nullableInt64(116),
        duration: nullableFloat64(6960),
      },
      video_streams: [videoStream()],
      audio_streams: [
        audioStream({ title: nullableString("English Stereo") }),
        audioStream({
          id: 2,
          stream_index: 2,
          language: nullableString("spa"),
          title: nullableString("Spanish Stereo"),
        }),
      ],
      subtitles: [sdhSubtitle()],
      chapters: [],
    },
  };
}

describe("PlaybackSettingsDialog", () => {
  it("renders labelled controls inside the media dialog surface", () => {
    const queryClient = createTestQueryClient();
    queryClient.setQueryData([MOVIE_TECHNICAL_DETAILS_KEY, 22], technicalDetails());

    renderWithQueryClient(
      <PlaybackSettingsDialog
        movieId={22}
        open
        onOpenChange={vi.fn()}
        settings={{ mode: "direct", audioTrack: 0, subtitleTrack: null }}
        onSave={vi.fn()}
      />,
      { queryClient },
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
    const queryClient = createTestQueryClient();
    queryClient.setQueryData(
      [MOVIE_TECHNICAL_DETAILS_KEY, 22],
      technicalDetails(),
    );
    const onSave = vi.fn();
    const onOpenChange = vi.fn();

    renderWithQueryClient(
      <PlaybackSettingsDialog
        movieId={22}
        open
        onOpenChange={onOpenChange}
        settings={{ mode: "direct", audioTrack: 0, subtitleTrack: null }}
        onSave={onSave}
      />,
      { queryClient },
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
    const queryClient = createTestQueryClient();
    const details = technicalDetails();
    details.data!.subtitles.push({
      ...sdhSubtitle(),
      id: 2,
      stream_index: 4,
      codec: "hdmv_pgs_subtitle",
      title: nullableString("Signs"),
    });
    queryClient.setQueryData([MOVIE_TECHNICAL_DETAILS_KEY, 22], details);

    renderWithQueryClient(
      <PlaybackSettingsDialog
        movieId={22}
        open
        onOpenChange={vi.fn()}
        settings={{ mode: "direct", audioTrack: 0, subtitleTrack: null }}
        onSave={vi.fn()}
      />,
      { queryClient },
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
    const queryClient = createTestQueryClient();

    renderWithQueryClient(
      <PlaybackSettingsDialog
        movieId={22}
        open
        onOpenChange={vi.fn()}
        settings={{ mode: "direct", audioTrack: 0, subtitleTrack: null }}
        onSave={vi.fn()}
      />,
      { queryClient },
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
    const queryClient = createTestQueryClient();
    const details = technicalDetails();
    details.data!.movie.mime_type = "video/x-matroska";
    details.data!.movie.container = "mkv";
    details.data!.video_streams[0].codec = "hevc";
    queryClient.setQueryData([MOVIE_TECHNICAL_DETAILS_KEY, 22], details);

    renderWithQueryClient(
      <PlaybackSettingsDialog
        movieId={22}
        open
        onOpenChange={vi.fn()}
        settings={{ mode: "direct", audioTrack: 0, subtitleTrack: null }}
        onSave={vi.fn()}
      />,
      { queryClient },
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
      const queryClient = createTestQueryClient();
      const details = technicalDetails();
      details.data!.movie.mime_type = "video/x-matroska";
      details.data!.movie.container = "mkv";
      details.data!.video_streams = [];
      queryClient.setQueryData([MOVIE_TECHNICAL_DETAILS_KEY, 22], details);
      const onSave = vi.fn();

      renderWithQueryClient(
        <PlaybackSettingsDialog
          movieId={22}
          open
          onOpenChange={vi.fn()}
          settings={{ mode: "direct", audioTrack: 0, subtitleTrack: null }}
          onSave={onSave}
        />,
        { queryClient },
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
    const queryClient = createTestQueryClient();
    queryClient.setQueryData(
      [MOVIE_TECHNICAL_DETAILS_KEY, 22],
      technicalDetails(),
    );
    const onSave = vi.fn();

    renderWithQueryClient(
      <PlaybackSettingsDialog
        movieId={22}
        open
        onOpenChange={vi.fn()}
        settings={{ mode: "direct", audioTrack: 0, subtitleTrack: null }}
        onSave={onSave}
      />,
      { queryClient },
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
    const queryClient = createTestQueryClient();
    queryClient.setQueryData(
      [MOVIE_TECHNICAL_DETAILS_KEY, 22],
      technicalDetails(),
    );
    const onSave = vi.fn();

    renderWithQueryClient(
      <PlaybackSettingsDialog
        movieId={22}
        open
        onOpenChange={vi.fn()}
        settings={{ mode: "remux", audioTrack: 1, subtitleTrack: null }}
        onSave={onSave}
      />,
      { queryClient },
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
