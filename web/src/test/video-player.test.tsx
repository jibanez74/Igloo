import { createRef } from "react";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import VideoPlayer from "@/components/VideoPlayer";
import { MOVIE_BUFFERING_SPINNER_DELAY_MS } from "@/lib/constants";

// jsdom does not implement HTMLMediaElement.load (called by the native-source
// cleanup path) or HTMLTrackElement.track (assigned `mode = "showing"` when a
// subtitle track mounts), so both are polyfilled for these tests.
const trackObjects = new WeakMap<HTMLTrackElement, { mode: string }>();
let originalTrackDescriptor: PropertyDescriptor | undefined;

beforeAll(() => {
  window.HTMLMediaElement.prototype.load = vi.fn();
  originalTrackDescriptor = Object.getOwnPropertyDescriptor(
    window.HTMLTrackElement.prototype,
    "track",
  );
  Object.defineProperty(window.HTMLTrackElement.prototype, "track", {
    configurable: true,
    get() {
      let value = trackObjects.get(this as HTMLTrackElement);
      if (!value) {
        value = { mode: "disabled" };
        trackObjects.set(this as HTMLTrackElement, value);
      }
      return value;
    },
  });
});

afterAll(() => {
  if (originalTrackDescriptor) {
    Object.defineProperty(
      window.HTMLTrackElement.prototype,
      "track",
      originalTrackDescriptor,
    );
  } else {
    delete (window.HTMLTrackElement.prototype as { track?: unknown }).track;
  }
});

afterEach(() => {
  vi.useRealTimers();
});

function renderPlayer(
  props: Partial<React.ComponentProps<typeof VideoPlayer>> = {},
) {
  const videoRef = createRef<HTMLVideoElement>();
  const result = render(
    <VideoPlayer
      videoRef={videoRef}
      src="/api/movies/1/stream"
      title="Test Movie"
      onError={vi.fn()}
      {...props}
    />,
  );
  const video = screen.getByLabelText("Video player for Test Movie");
  return { ...result, video };
}

describe("VideoPlayer subtitle track", () => {
  it("injects a showing track element for the active subtitle", () => {
    const { video } = renderPlayer({
      subtitleTrack: {
        url: "/api/movies/1/subtitles/2/web.vtt",
        label: "English",
        srclang: "en",
      },
    });

    const track = video.querySelector<HTMLTrackElement>("track[data-subtitle]");
    expect(track).not.toBeNull();
    expect(track).toHaveAttribute("kind", "subtitles");
    expect(track).toHaveAttribute("src", "/api/movies/1/subtitles/2/web.vtt");
    expect(track).toHaveAttribute("srclang", "en");
    expect(track).toHaveAttribute("label", "English");
    expect(track?.track.mode).toBe("showing");
  });

  it("swaps the track element when the subtitle URL changes", () => {
    const { video, rerender } = renderPlayer({
      subtitleTrack: {
        url: "/api/movies/1/subtitles/2/web.vtt",
        label: "English",
        srclang: "en",
      },
    });

    rerender(
      <VideoPlayer
        videoRef={createRef<HTMLVideoElement>()}
        src="/api/movies/1/stream"
        title="Test Movie"
        onError={vi.fn()}
        subtitleTrack={{
          url: "/api/movies/1/subtitles/3/web.vtt",
          label: "Spanish",
          srclang: "es",
        }}
      />,
    );

    const tracks = video.querySelectorAll("track[data-subtitle]");
    expect(tracks).toHaveLength(1);
    expect(tracks[0]).toHaveAttribute("src", "/api/movies/1/subtitles/3/web.vtt");
  });

  it("removes the track element when subtitles are turned off", () => {
    const { video, rerender } = renderPlayer({
      subtitleTrack: {
        url: "/api/movies/1/subtitles/2/web.vtt",
        label: "English",
        srclang: "en",
      },
    });

    rerender(
      <VideoPlayer
        videoRef={createRef<HTMLVideoElement>()}
        src="/api/movies/1/stream"
        title="Test Movie"
        onError={vi.fn()}
        subtitleTrack={null}
      />,
    );

    expect(video.querySelector("track[data-subtitle]")).toBeNull();
  });
});

describe("VideoPlayer buffering indicator", () => {
  it("shows the spinner after the delay and clears it when playback resumes", async () => {
    vi.useFakeTimers();
    const { video } = renderPlayer();

    fireEvent(video, new Event("waiting"));
    expect(screen.queryByRole("status", { name: "Buffering" })).toBeNull();

    await act(async () => {
      await vi.advanceTimersByTimeAsync(MOVIE_BUFFERING_SPINNER_DELAY_MS);
    });
    expect(screen.getByRole("status", { name: "Buffering" })).toBeInTheDocument();

    fireEvent(video, new Event("playing"));
    expect(screen.queryByRole("status", { name: "Buffering" })).toBeNull();
  });

  it("never shows the spinner for a stall shorter than the delay", async () => {
    vi.useFakeTimers();
    const { video } = renderPlayer();

    fireEvent(video, new Event("waiting"));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(MOVIE_BUFFERING_SPINNER_DELAY_MS - 1);
    });
    fireEvent(video, new Event("playing"));

    await act(async () => {
      await vi.advanceTimersByTimeAsync(MOVIE_BUFFERING_SPINNER_DELAY_MS);
    });
    expect(screen.queryByRole("status", { name: "Buffering" })).toBeNull();
  });

  it("treats a stalled event as buffering and clears on pause", async () => {
    vi.useFakeTimers();
    const { video } = renderPlayer({ onPause: vi.fn() });

    fireEvent(video, new Event("stalled"));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(MOVIE_BUFFERING_SPINNER_DELAY_MS);
    });
    expect(screen.getByRole("status", { name: "Buffering" })).toBeInTheDocument();

    fireEvent(video, new Event("pause"));
    expect(screen.queryByRole("status", { name: "Buffering" })).toBeNull();
  });

  it("does not block pointer events over the video surface", async () => {
    vi.useFakeTimers();
    const { video } = renderPlayer();

    fireEvent(video, new Event("seeking"));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(MOVIE_BUFFERING_SPINNER_DELAY_MS);
    });

    const overlay = screen
      .getByRole("status", { name: "Buffering" })
      .closest("div.pointer-events-none");
    expect(overlay).not.toBeNull();
  });

  it("forwards pause and ended to the parent callbacks", () => {
    const onPause = vi.fn();
    const onEnded = vi.fn();
    const { video } = renderPlayer({ onPause, onEnded });

    fireEvent(video, new Event("pause"));
    fireEvent(video, new Event("ended"));

    expect(onPause).toHaveBeenCalledTimes(1);
    expect(onEnded).toHaveBeenCalledTimes(1);
  });
});
