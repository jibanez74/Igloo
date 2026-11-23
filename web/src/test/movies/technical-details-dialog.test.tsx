import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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
import { jsonResponse } from "../helpers/api";
import {
  nullableFloat64,
  nullableInt64,
  nullableString,
} from "../helpers/fixtures";
import { createTestQueryClient, renderWithQueryClient } from "../helpers/render";
import {
  audioStream,
  subtitleStream,
  videoStream,
} from "../helpers/tech-details";

type Details = MovieTechnicalDetailsResponse;

function technicalDetails(
  runtimeMinutes: number | null,
  overrides: Partial<Omit<Details, "movie">> & {
    movie?: Partial<Details["movie"]>;
  } = {},
): ApiResponseType<Details> {
  const { movie, ...streams } = overrides;
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
        ...movie,
      },
      video_streams: [],
      audio_streams: [],
      subtitles: [],
      chapters: [],
      ...streams,
    },
  };
}

function renderDialog(
  runtimeMinutes: number | null,
  overrides: Parameters<typeof technicalDetails>[1] = {},
) {
  const queryClient = createTestQueryClient();
  queryClient.setQueryData(
    [MOVIE_TECHNICAL_DETAILS_KEY, 22],
    technicalDetails(runtimeMinutes, overrides),
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
  it("reports a failed load with Try again instead of an empty body", async () => {
    const user = userEvent.setup();
    const queryClient = createTestQueryClient();
    queryClient.setQueryData([MOVIE_TECHNICAL_DETAILS_KEY, 22], {
      error: true,
      message: "failed to read streams",
      status: 500,
    });
    vi.stubGlobal(
      "fetch",
      vi.fn(() => jsonResponse(technicalDetails(116))),
    );

    renderWithQueryClient(
      <TechnicalDetailsDialog movieId={22} open onOpenChange={vi.fn()} />,
      { queryClient },
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Failed to load technical details.",
    );
    await user.click(screen.getByRole("button", { name: "Try again" }));
    expect(await screen.findByText("arrival.mp4")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

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
    // A scroll region with no focusable content is itself a tab stop.
    expect(body).toHaveAttribute("tabindex", "0");
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

  it("describes each stream with every value the scanner reported", () => {
    renderDialog(116, {
      video_streams: [
        videoStream({
          codec: "hevc",
          codec_profile: nullableString("Main 10"),
          width: 3840,
          height: 2160,
          frame_rate: 23.976,
          bit_rate: 25_000_000,
          bit_depth: nullableInt64(10),
          color_space: nullableString("bt2020nc"),
          language: nullableString("eng"),
          title: nullableString("Main feature"),
        }),
      ],
      audio_streams: [
        audioStream({
          codec: "eac3",
          codec_profile: nullableString("Dolby Digital Plus"),
          channels: 6,
          channel_layout: nullableString("5.1(side)"),
          sample_rate: nullableInt64(48_000),
          bit_rate: 640_000,
          title: nullableString("Surround"),
        }),
      ],
      subtitles: [subtitleStream({ is_default: true, is_forced: true })],
    });

    const video = screen.getByRole("region", { name: "Video streams" });
    expect(detailRows(video)).toEqual({
      Codec: "hevc (Main 10)",
      Resolution: "3840×2160",
      "Aspect ratio": "16:9",
      "Frame rate": "23.98 fps",
      "Bit rate": "25000 kbps",
      "Bit depth": "10-bit",
      "Color space": "bt2020nc",
    });
    expect(within(video).getByRole("heading", { level: 4 })).toHaveTextContent(
      "Video stream 0eng(Main feature)",
    );

    const audio = screen.getByRole("region", { name: "Audio streams" });
    expect(detailRows(audio)).toEqual({
      Codec: "eac3 (Dolby Digital Plus)",
      Channels: "6 (5.1(side))",
      "Sample rate": "48000 Hz",
      "Bit rate": "640 kbps",
    });

    const subtitles = screen.getByRole("region", { name: "Subtitle streams" });
    expect(detailRows(subtitles)).toEqual({
      Codec: "subrip",
      Flags: "Default, Forced",
    });
    expect(
      screen.queryByText("No video, audio, or subtitle streams were reported for this file."),
    ).not.toBeInTheDocument();
  });

  it("leaves out the values the scanner did not report", () => {
    renderDialog(116, {
      video_streams: [
        videoStream({
          codec_profile: nullableString(),
          aspect_ratio: nullableString(),
          bit_rate: 0,
          bit_depth: nullableInt64(),
        }),
      ],
      audio_streams: [
        audioStream({
          codec_profile: nullableString(),
          channel_layout: nullableString(),
          sample_rate: nullableInt64(),
          bit_rate: 0,
        }),
      ],
      subtitles: [subtitleStream({ language: nullableString() })],
    });

    expect(detailRows(screen.getByRole("region", { name: "Video streams" }))).toEqual({
      Codec: "h264",
      Resolution: "1920×1080",
      "Frame rate": "24.00 fps",
    });
    expect(detailRows(screen.getByRole("region", { name: "Audio streams" }))).toEqual({
      Codec: "aac",
      Channels: "2",
    });
    const subtitles = screen.getByRole("region", { name: "Subtitle streams" });
    expect(detailRows(subtitles)).toEqual({ Codec: "subrip" });
    expect(within(subtitles).getByRole("heading", { level: 4 })).toHaveTextContent(
      /^Subtitle stream 2$/,
    );
  });

  it("counts the streams of a kind when there is more than one", () => {
    renderDialog(116, {
      audio_streams: [audioStream({ id: 1 }), audioStream({ id: 2, stream_index: 2 })],
      subtitles: [subtitleStream()],
    });

    const audio = screen.getByRole("region", { name: "Audio streams, 2 streams" });
    expect(within(audio).getByRole("heading", { level: 3 })).toHaveTextContent(
      "Audio streams (2)",
    );
    expect(within(audio).getAllByRole("listitem")).toHaveLength(2);
    expect(
      screen.getByRole("region", { name: "Subtitle streams" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("region", { name: /^Video streams/ })).not.toBeInTheDocument();
  });

  it("does not call a file with only audio streams empty", () => {
    renderDialog(116, { audio_streams: [audioStream()] });

    expect(screen.getByRole("region", { name: "Audio streams" })).toBeInTheDocument();
    expect(
      screen.queryByText("No video, audio, or subtitle streams were reported for this file."),
    ).not.toBeInTheDocument();
  });

  it("says so when the file reported no streams", () => {
    renderDialog(116);

    expect(
      screen.getByText("No video, audio, or subtitle streams were reported for this file."),
    ).toBeInTheDocument();
  });

  it.each([
    [512, "512 B"],
    // Binary units: 1000 bytes is still under a kilobyte.
    [1000, "1000 B"],
    [1536, "1.5 KB"],
    [5 * 1024 * 1024, "5.0 MB"],
    [3.5 * 1024 * 1024 * 1024, "3.50 GB"],
  ])("shows a %d-byte file as %s", (size, label) => {
    renderDialog(116, { movie: { size, container: "mkv" } });

    expect(detailRows(screen.getByRole("region", { name: "File" }))).toMatchObject({
      "File size": label,
      "Container format": "MKV",
    });
  });
});

/** The label/value rows of one dialog section, as a label → value map. */
function detailRows(section: HTMLElement) {
  return Object.fromEntries(
    within(section)
      .getAllByText((_, element) => element?.tagName === "P")
      .map(row => {
        const [label, value] = Array.from(row.children);
        return [label.textContent, value.textContent];
      }),
  );
}
