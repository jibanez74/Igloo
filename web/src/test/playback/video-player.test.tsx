import { createRef, StrictMode, useRef, useState } from "react";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import VideoPlayer from "@/components/playback/VideoPlayer";
import {
  HLS_CAPACITY_RETRY_FALLBACK_SEC,
  HLS_JS_LOAD_TIMEOUT_MS,
  HLS_NETWORK_RECOVERY_DELAYS_MS,
  HLS_SEEK_SETTLE_MS,
  HLS_SEGMENT_NOT_READY_MAX_RETRIES,
  MOVIE_BUFFERING_SPINNER_DELAY_MS,
} from "@/lib/constants";

type FakeHlsListener = (event: string, data: unknown) => void;

const fakeHlsInstances = vi.hoisted(() => [] as FakeHlsInstance[]);
const fakeHlsSupport = vi.hoisted(() => ({ supported: true }));
const nativeHlsSupport = vi.hoisted(() => ({ supported: false }));

type FakeLevelDetails = { live: boolean; endSN: number };

type FakeHlsInstance = {
  listeners: Map<string, FakeHlsListener[]>;
  trigger: (event: string, data: unknown) => void;
  /** Every event the component itself raised through `trigger`. */
  triggered: Array<{ event: string; data: unknown }>;
  levels: Array<{ details?: FakeLevelDetails }>;
  destroyed: boolean;
  startLoadCalls: number;
  loadSourceCalls: number;
  stopLoadCalls: number;
  recoverMediaErrorCalls: number;
};

vi.mock("@/lib/playback", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/playback")>();
  return {
    ...actual,
    get prefersNativeHLS() {
      return nativeHlsSupport.supported;
    },
  };
});

vi.mock("hls.js/light", () => {
  class FakeHls implements FakeHlsInstance {
    static isSupported() {
      return fakeHlsSupport.supported;
    }
    static Events = {
      ERROR: "hlsError",
      MANIFEST_PARSED: "hlsManifestParsed",
      MANIFEST_LOADED: "hlsManifestLoaded",
      FRAG_BUFFERED: "hlsFragBuffered",
      BUFFER_EOS: "hlsBufferEos",
    };
    static ErrorDetails = {
      FRAG_LOAD_ERROR: "fragLoadError",
      MANIFEST_LOAD_ERROR: "manifestLoadError",
      LEVEL_LOAD_ERROR: "levelLoadError",
    };

    listeners = new Map<string, FakeHlsListener[]>();
    triggered: Array<{ event: string; data: unknown }> = [];
    levels: Array<{ details?: FakeLevelDetails }> = [];
    destroyed = false;
    startLoadCalls = 0;
    loadSourceCalls = 0;
    stopLoadCalls = 0;
    recoverMediaErrorCalls = 0;

    constructor() {
      fakeHlsInstances.push(this);
    }

    on(event: string, listener: FakeHlsListener) {
      const existing = this.listeners.get(event) ?? [];
      this.listeners.set(event, [...existing, listener]);
    }

    once(event: string, listener: FakeHlsListener) {
      this.on(event, listener);
    }

    trigger(event: string, data: unknown) {
      this.triggered.push({ event, data });
      for (const listener of this.listeners.get(event) ?? []) {
        listener(event, data);
      }
    }

    loadSource() {
      this.loadSourceCalls += 1;
    }
    attachMedia() {}
    recoverMediaError() {
      this.recoverMediaErrorCalls += 1;
    }
    startLoad() {
      this.startLoadCalls += 1;
    }
    stopLoad() {
      this.stopLoadCalls += 1;
    }
    destroy() {
      this.destroyed = true;
    }
  }

  return { default: FakeHls };
});

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
  fakeHlsInstances.length = 0;
  fakeHlsSupport.supported = true;
  nativeHlsSupport.supported = false;
});

function renderPlayer(
  props: Partial<React.ComponentProps<typeof VideoPlayer>> = {},
) {
  const videoRef = createRef<HTMLVideoElement>();
  const result = render(
    <VideoPlayer
      videoRef={videoRef}
      src="/api/movies/1/stream"
      isHlsSource={false}
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
        isHlsSource={false}
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
        isHlsSource={false}
        title="Test Movie"
        onError={vi.fn()}
        subtitleTrack={null}
      />,
    );

    expect(video.querySelector("track[data-subtitle]")).toBeNull();
  });
});

describe("VideoPlayer source lifecycle on start changes", () => {
  // Audit D10: the direct-play URL is a constant, so a start change (resume
  // dialog navigation) must seek, not tear the source down and refetch the
  // file from byte 0.
  it("does not reload a direct source when only startSec changes", () => {
    const videoRef = createRef<HTMLVideoElement>();
    const loadMock = vi.mocked(window.HTMLMediaElement.prototype.load);
    const baseProps = {
      videoRef,
      src: "/api/movies/1/stream",
      isHlsSource: false,
      title: "Test Movie",
      onError: vi.fn(),
      subtitleTrack: {
        url: "/api/movies/1/subtitles/2/web.vtt",
        label: "English",
        srclang: "en",
      },
    };
    const { rerender } = render(<VideoPlayer {...baseProps} startSec={0} />);
    const video = screen.getByLabelText("Video player for Test Movie");
    expect(video).toHaveAttribute("src", "/api/movies/1/stream");
    const loadCallsAfterMount = loadMock.mock.calls.length;

    rerender(<VideoPlayer {...baseProps} startSec={30} />);

    expect(loadMock.mock.calls.length).toBe(loadCallsAfterMount);
    expect(video).toHaveAttribute("src", "/api/movies/1/stream");
    // Audit D11 guard: the subtitle track must stay enabled across the change.
    const track = video.querySelector<HTMLTrackElement>("track[data-subtitle]");
    expect(track?.track.mode).toBe("showing");
  });

  // H10: dispose must tear down exactly the instance it belongs to. The
  // identity guard inside disposeHls (only null hlsRef when it still points
  // at the disposed instance) is not separately observable here, but this
  // pins the surrounding contract: a rebuild destroys the replaced instance,
  // never the live one, and unmount destroys the survivor.
  it("destroys only the replaced instance when the source changes", async () => {
    const videoRef = createRef<HTMLVideoElement>();
    const baseProps = {
      videoRef,
      isHlsSource: true,
      title: "Test Movie",
      onError: vi.fn(),
    };
    const { rerender, unmount } = render(
      <VideoPlayer
        {...baseProps}
        src="/api/movies/1/hls/remux/playlist.m3u8?playback_session=a&start=0"
      />,
    );
    await act(async () => {});
    expect(fakeHlsInstances).toHaveLength(1);

    rerender(
      <VideoPlayer
        {...baseProps}
        src="/api/movies/1/hls/remux/playlist.m3u8?playback_session=b&start=0"
      />,
    );
    await act(async () => {});
    expect(fakeHlsInstances).toHaveLength(2);
    expect(fakeHlsInstances[0].destroyed).toBe(true);
    expect(fakeHlsInstances[1].destroyed).toBe(false);

    unmount();
    expect(fakeHlsInstances[1].destroyed).toBe(true);
  });

  // The fallback seek effect used to gate on hlsRef, which is assigned
  // asynchronously and still null when the effect runs on a fresh hls.js
  // mount — so its loadedmetadata listener competed with hls.js's own
  // startPosition seek. The gate is by source type now.
  it("does not compete with hls.js startPosition on a fresh mount", async () => {
    const onStartApplied = vi.fn();
    const videoRef = createRef<HTMLVideoElement>();
    render(
      <VideoPlayer
        videoRef={videoRef}
        src="/api/movies/1/hls/remux/playlist.m3u8?playback_session=a&start=30"
        isHlsSource
        title="Test Movie"
        onError={vi.fn()}
        startSec={30}
        onStartApplied={onStartApplied}
      />,
    );
    await act(async () => {});
    expect(fakeHlsInstances).toHaveLength(1);
    const video = screen.getByLabelText(
      "Video player for Test Movie",
    ) as HTMLVideoElement;

    fireEvent(video, new Event("loadedmetadata"));

    expect(onStartApplied).not.toHaveBeenCalled();
    expect(video.currentTime).toBe(0);
  });

  // Lock-in: for hls.js the URL can stay identical while startSec changes (a
  // resume target inside the rewind buffer keeps start=0 in the URL), and the
  // rebuild with startPosition is what applies that seek. It must survive.
  it("rebuilds the hls.js instance when startSec changes on the same URL", async () => {
    const videoRef = createRef<HTMLVideoElement>();
    const baseProps = {
      videoRef,
      src: "/api/movies/1/hls/remux/playlist.m3u8?playback_session=uuid&start=0",
      isHlsSource: true,
      title: "Test Movie",
      onError: vi.fn(),
    };
    const { rerender } = render(<VideoPlayer {...baseProps} startSec={5} />);
    await act(async () => {});
    expect(fakeHlsInstances).toHaveLength(1);

    rerender(<VideoPlayer {...baseProps} startSec={8} />);
    await act(async () => {});
    expect(fakeHlsInstances).toHaveLength(2);
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

describe("VideoPlayer hls.js error routing", () => {
  const hlsSrc =
    "/api/movies/1/hls/remux/playlist.m3u8?playback_session=uuid&start=0";

  async function renderHlsPlayer(
    props: Partial<React.ComponentProps<typeof VideoPlayer>> = {},
  ) {
    renderPlayer({ src: hlsSrc, isHlsSource: true, ...props });
    // The hls.js module loads through a dynamic import; flush it.
    await act(async () => {});
    expect(fakeHlsInstances).toHaveLength(1);
    return fakeHlsInstances[0];
  }

  it("reports 503 manifest errors as capacity-busy with the Retry-After delay", async () => {
    const onCapacityBusy = vi.fn();
    const onError = vi.fn();
    const hls = await renderHlsPlayer({ onCapacityBusy, onError });

    act(() => {
      hls.trigger("hlsError", {
        type: "networkError",
        details: "manifestLoadError",
        fatal: true,
        response: { code: 503 },
        networkDetails: {
          getResponseHeader: (name: string) =>
            name === "Retry-After" ? "7" : null,
        },
      });
    });

    expect(onCapacityBusy).toHaveBeenCalledWith(7);
    expect(onError).not.toHaveBeenCalled();
  });

  it("falls back to the default delay when Retry-After is missing", async () => {
    const onCapacityBusy = vi.fn();
    const hls = await renderHlsPlayer({ onCapacityBusy });

    act(() => {
      hls.trigger("hlsError", {
        type: "networkError",
        details: "levelLoadError",
        fatal: true,
        response: { code: 503 },
        networkDetails: null,
      });
    });

    expect(onCapacityBusy).toHaveBeenCalledWith(
      HLS_CAPACITY_RETRY_FALLBACK_SEC,
    );
  });

  it("leaves nonfatal level-load 503 errors with hls.js", async () => {
    const onCapacityBusy = vi.fn();
    const onError = vi.fn();
    const hls = await renderHlsPlayer({ onCapacityBusy, onError });

    act(() => {
      hls.trigger("hlsError", {
        type: "networkError",
        details: "levelLoadError",
        fatal: false,
        response: { code: 503 },
      });
    });

    expect(onCapacityBusy).not.toHaveBeenCalled();
    expect(onError).not.toHaveBeenCalled();
  });

  it("surfaces a 503 as a fatal error when no capacity handler is wired", async () => {
    const onError = vi.fn();
    const hls = await renderHlsPlayer({ onError });

    act(() => {
      hls.trigger("hlsError", {
        type: "networkError",
        details: "manifestLoadError",
        fatal: true,
        response: { code: 503 },
      });
    });

    expect(onError).toHaveBeenCalledOnce();
  });

  it("still routes fragment 404s to onSessionLost", async () => {
    const onSessionLost = vi.fn();
    const onCapacityBusy = vi.fn();
    const hls = await renderHlsPlayer({ onSessionLost, onCapacityBusy });

    act(() => {
      hls.trigger("hlsError", {
        type: "networkError",
        details: "fragLoadError",
        fatal: false,
        response: { code: 404 },
      });
    });

    expect(onSessionLost).toHaveBeenCalledOnce();
    expect(onCapacityBusy).not.toHaveBeenCalled();
  });

  const pastEndDetails = {
    getResponseHeader: (name: string) =>
      name === "X-Igloo-Segment" ? "past-end" : null,
  };

  // The server marks a segment 404 as past the end only when FFmpeg exited
  // cleanly without writing it: the synthesized transcode playlist can list
  // one or two segments more than FFmpeg writes when a source's audio
  // outlasts its video. Everything before them is buffered, so the marked
  // 404 ends the media instead of rebasing the session.
  it("treats a segment 404 marked past-end as end of stream", async () => {
    const onSessionLost = vi.fn();
    const onError = vi.fn();
    const hls = await renderHlsPlayer({ onSessionLost, onError });

    act(() => {
      hls.trigger("hlsError", {
        type: "networkError",
        details: "fragLoadError",
        fatal: false,
        response: { code: 404 },
        networkDetails: pastEndDetails,
        frag: { sn: 9, level: 0 },
      });
    });

    expect(hls.stopLoadCalls).toBe(1);
    expect(hls.triggered).toContainEqual({ event: "hlsBufferEos", data: { type: null } });
    expect(onSessionLost).not.toHaveBeenCalled();
    expect(onError).not.toHaveBeenCalled();
  });

  // Stopping hls.js at the end must not strand a viewer who then jumps back
  // past the back buffer: nothing would ever load again.
  it("resumes loading when the viewer seeks after an end-of-stream 404", async () => {
    const onSessionLost = vi.fn();
    const hls = await renderHlsPlayer({ onSessionLost });
    const video = screen.getByLabelText("Video player for Test Movie");

    act(() => {
      hls.trigger("hlsError", {
        type: "networkError",
        details: "fragLoadError",
        fatal: false,
        response: { code: 404 },
        networkDetails: pastEndDetails,
        frag: { sn: 10, level: 0 },
      });
    });
    expect(hls.startLoadCalls).toBe(0);

    fireEvent(video, new Event("seeking"));
    expect(hls.startLoadCalls).toBe(1);

    // Only the first seek re-arms; a normal seek afterwards is hls.js's own.
    fireEvent(video, new Event("seeking"));
    expect(hls.startLoadCalls).toBe(1);
    expect(onSessionLost).not.toHaveBeenCalled();
  });

  // The segment index proves nothing: an unmarked 404 on the last listed
  // segment is what an idle eviction looks like, and rebasing is the only
  // way to keep playing.
  it.each([
    ["the last segment of an ended playlist", { getResponseHeader: () => null }],
    ["a segment with no response details", null],
  ])(
    "still routes an unmarked 404 on %s to onSessionLost",
    async (_label, networkDetails) => {
      const onSessionLost = vi.fn();
      const hls = await renderHlsPlayer({ onSessionLost });
      hls.levels = [{ details: { live: false, endSN: 10 } }];

      act(() => {
        hls.trigger("hlsError", {
          type: "networkError",
          details: "fragLoadError",
          fatal: false,
          response: { code: 404 },
          networkDetails,
          frag: { sn: 10, level: 0 },
        });
      });

      expect(hls.stopLoadCalls).toBe(0);
      expect(onSessionLost).toHaveBeenCalledOnce();
    },
  );

  // The server 404s the playlist handler as well as the segment handler when a
  // session is gone, and hls.js reports those as manifest/level errors. Only
  // matching the fragment detail sent them to the error screen with no attempt
  // to recover the session.
  it.each(["manifestLoadError", "levelLoadError"])(
    "routes a %s 404 to onSessionLost",
    async (details) => {
      const onSessionLost = vi.fn();
      const onError = vi.fn();
      const hls = await renderHlsPlayer({ onSessionLost, onError });

      act(() => {
        hls.trigger("hlsError", {
          type: "networkError",
          details,
          fatal: true,
          response: { code: 404 },
        });
      });

      expect(onSessionLost).toHaveBeenCalledOnce();
      expect(onError).not.toHaveBeenCalled();
    },
  );

  // A 503 on a fragment means the encoder has not reached that segment yet, not
  // that the stream is broken. hls.js has spent its own retries by the time the
  // error is fatal, so the player grants a bounded number of fresh attempts.
  it("retries a not-yet-produced segment before reporting it", async () => {
    const onError = vi.fn();
    const hls = await renderHlsPlayer({ onError });

    const notReady = {
      type: "networkError",
      details: "fragLoadError",
      fatal: true,
      response: { code: 503 },
    };

    for (let attempt = 0; attempt < HLS_SEGMENT_NOT_READY_MAX_RETRIES; attempt++) {
      act(() => {
        hls.trigger("hlsError", notReady);
      });
    }

    expect(hls.startLoadCalls).toBe(HLS_SEGMENT_NOT_READY_MAX_RETRIES);
    expect(onError).not.toHaveBeenCalled();

    act(() => {
      hls.trigger("hlsError", notReady);
    });

    expect(hls.startLoadCalls).toBe(HLS_SEGMENT_NOT_READY_MAX_RETRIES);
    expect(onError).toHaveBeenCalledOnce();
    expect(onError.mock.calls[0][0]).toMatch(/still preparing/i);
  });

  // A buffered fragment proves the stream recovered, so the one-shot budgets
  // cover consecutive failures rather than the life of the instance.
  it("restores the retry budget once a fragment buffers", async () => {
    const onError = vi.fn();
    const hls = await renderHlsPlayer({ onError });

    const notReady = {
      type: "networkError",
      details: "fragLoadError",
      fatal: true,
      response: { code: 503 },
    };

    for (let attempt = 0; attempt < HLS_SEGMENT_NOT_READY_MAX_RETRIES; attempt++) {
      act(() => {
        hls.trigger("hlsError", notReady);
      });
    }
    act(() => {
      hls.trigger("hlsFragBuffered", {});
    });
    act(() => {
      hls.trigger("hlsError", notReady);
    });

    expect(hls.startLoadCalls).toBe(HLS_SEGMENT_NOT_READY_MAX_RETRIES + 1);
    expect(onError).not.toHaveBeenCalled();
  });

  // hls.js's documented recovery for a fatal network error. A restarting
  // server needs more than one immediate retry, so the retries back off
  // before the error screen shows.
  it("retries a load that got no answer with a backoff before reporting it", async () => {
    vi.useFakeTimers();
    const onError = vi.fn();
    const hls = await renderHlsPlayer({ onError });

    const networkFailure = {
      type: "networkError",
      details: "fragLoadTimeOut",
      fatal: true,
    };

    for (const [attempt, delayMs] of HLS_NETWORK_RECOVERY_DELAYS_MS.entries()) {
      act(() => {
        hls.trigger("hlsError", networkFailure);
      });
      await act(async () => {
        await vi.advanceTimersByTimeAsync(delayMs - 1);
      });
      expect(hls.startLoadCalls).toBe(attempt);
      await act(async () => {
        await vi.advanceTimersByTimeAsync(1);
      });
      expect(hls.startLoadCalls).toBe(attempt + 1);
    }
    expect(onError).not.toHaveBeenCalled();

    act(() => {
      hls.trigger("hlsError", networkFailure);
    });

    expect(hls.startLoadCalls).toBe(HLS_NETWORK_RECOVERY_DELAYS_MS.length);
    expect(onError).toHaveBeenCalledOnce();
  });

  // The XHR loader reports a dropped connection, which is also what a
  // restarting server looks like, as status 0 rather than no response. Only
  // timeouts used to reach the retry; this went straight to the error screen.
  it("treats a dropped connection (status 0) as a load with no answer", async () => {
    vi.useFakeTimers();
    const onError = vi.fn();
    const hls = await renderHlsPlayer({ onError });

    act(() => {
      hls.trigger("hlsError", {
        type: "networkError",
        details: "fragLoadError",
        fatal: true,
        response: { code: 0 },
      });
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(HLS_NETWORK_RECOVERY_DELAYS_MS[0]);
    });

    expect(hls.startLoadCalls).toBe(1);
    expect(onError).not.toHaveBeenCalled();
  });

  it("fetches the manifest again when the manifest request got no answer", async () => {
    vi.useFakeTimers();
    const hls = await renderHlsPlayer();
    const initialLoads = hls.loadSourceCalls;

    act(() => {
      hls.trigger("hlsError", {
        type: "networkError",
        details: "manifestLoadError",
        fatal: true,
        response: { code: 0 },
      });
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(HLS_NETWORK_RECOVERY_DELAYS_MS[0]);
    });

    expect(hls.loadSourceCalls).toBe(initialLoads + 1);
    expect(hls.startLoadCalls).toBe(0);
  });

  it("cancels a scheduled network retry when the instance is replaced", async () => {
    vi.useFakeTimers();
    const hls = await renderHlsPlayer();

    act(() => {
      hls.trigger("hlsError", {
        type: "networkError",
        details: "fragLoadTimeOut",
        fatal: true,
      });
    });
    cleanup();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(HLS_NETWORK_RECOVERY_DELAYS_MS[0]);
    });

    expect(hls.destroyed).toBe(true);
    expect(hls.startLoadCalls).toBe(0);
  });

  // FFmpeg died partway through. The server replaces the failed session on
  // the next manifest request, so a rebase keeps the film playing.
  it("routes a fatal segment 500 to onSessionLost", async () => {
    const onSessionLost = vi.fn();
    const onError = vi.fn();
    const hls = await renderHlsPlayer({ onSessionLost, onError });

    act(() => {
      hls.trigger("hlsError", {
        type: "networkError",
        details: "fragLoadError",
        fatal: true,
        response: { code: 500 },
      });
    });

    expect(onSessionLost).toHaveBeenCalledOnce();
    expect(onError).not.toHaveBeenCalled();
  });

  // A lost session fails every request still in flight; each used to spend
  // one attempt of the recovery budget on the same failure.
  it("reports a lost session once per hls.js instance", async () => {
    const onSessionLost = vi.fn();
    const hls = await renderHlsPlayer({ onSessionLost });

    const lost = {
      type: "networkError",
      details: "fragLoadError",
      fatal: false,
      response: { code: 404 },
    };
    act(() => {
      hls.trigger("hlsError", lost);
      hls.trigger("hlsError", { ...lost, details: "levelLoadError" });
    });

    expect(onSessionLost).toHaveBeenCalledOnce();
  });

  // Before anything buffers, currentTime reads 0, the new session's start.
  // Reporting that sent every recovery ten seconds (the resume rewind)
  // before its target, so a 404 that kept coming back walked the film
  // backwards.
  it("reports the source's start position until a fragment buffers", async () => {
    const onSessionLost = vi.fn();
    const hls = await renderHlsPlayer({ onSessionLost, startSec: 30 });

    act(() => {
      hls.trigger("hlsError", {
        type: "networkError",
        details: "manifestLoadError",
        fatal: true,
        response: { code: 404 },
      });
    });

    expect(onSessionLost).toHaveBeenCalledWith(30);
  });

  it("reports the playhead once a fragment has buffered", async () => {
    const onSessionLost = vi.fn();
    const hls = await renderHlsPlayer({ onSessionLost, startSec: 30 });

    act(() => {
      hls.trigger("hlsFragBuffered", {});
      hls.trigger("hlsError", {
        type: "networkError",
        details: "fragLoadError",
        fatal: false,
        response: { code: 404 },
      });
    });

    // jsdom never advances the playhead, so it still reads 0.
    expect(onSessionLost).toHaveBeenCalledWith(0);
  });
});

describe("VideoPlayer HLS capability and effective profile", () => {
  const hlsSrc = "/api/movies/1/hls/remux/playlist.m3u8";

  async function renderHls(
    props: Partial<React.ComponentProps<typeof VideoPlayer>> = {},
  ) {
    const result = renderPlayer({ src: hlsSrc, isHlsSource: true, ...props });
    await act(async () => {});
    return result;
  }

  // Without this the player sat blank with no error at all, so a browser
  // lacking Media Source Extensions looked like a broken app (audit H11).
  it("reports an error when hls.js is unsupported", async () => {
    fakeHlsSupport.supported = false;
    const onError = vi.fn();

    await renderHls({ onError });

    expect(fakeHlsInstances).toHaveLength(0);
    expect(onError).toHaveBeenCalledOnce();
    expect(onError.mock.calls[0][0]).toMatch(/cannot play streamed video/i);
  });

  // A remux request that fails the server's safety gate is still served from
  // the /hls/remux/ path, so only the response header can report what actually
  // ran (audit H3).
  it("reports the effective profile from the manifest response header", async () => {
    const onEffectiveProfile = vi.fn();
    await renderHls({ onEffectiveProfile });

    const hls = fakeHlsInstances[0];
    act(() => {
      hls.trigger("hlsManifestLoaded", {
        networkDetails: {
          getResponseHeader: (name: string) =>
            name === "X-Igloo-Effective-Profile" ? "2160p_16mbps" : null,
        },
      });
    });

    expect(onEffectiveProfile).toHaveBeenCalledWith("2160p_16mbps");
  });

  it("stays quiet when the manifest response has no effective-profile header", async () => {
    const onEffectiveProfile = vi.fn();
    await renderHls({ onEffectiveProfile });

    const hls = fakeHlsInstances[0];
    act(() => {
      hls.trigger("hlsManifestLoaded", {
        networkDetails: { getResponseHeader: () => null },
      });
    });

    expect(onEffectiveProfile).not.toHaveBeenCalled();
  });

  it.each([
    ["valid", "581", 581],
    ["zero", "0", 0],
    ["missing", null, null],
    ["malformed", "581 seconds", null],
    ["negative", "-1", null],
    ["non-finite NaN", "NaN", null],
    ["non-finite infinity", "Infinity", null],
    ["later than requested", "590.001", null],
  ])(
    "validates a %s actual-start manifest header",
    async (_case, header, expected) => {
      const onActualStart = vi.fn();
      await renderHls({
        requestedStartSec: 590,
        onActualStart,
      });

      act(() => {
        fakeHlsInstances[0].trigger("hlsManifestLoaded", {
          networkDetails: {
            getResponseHeader: (name: string) =>
              name === "X-Igloo-Actual-Start" ? header : null,
          },
        });
      });

      if (expected === null) {
        expect(onActualStart).not.toHaveBeenCalled();
      } else {
        expect(onActualStart).toHaveBeenCalledOnce();
        expect(onActualStart).toHaveBeenCalledWith(expected);
      }
    },
  );

  it("reads both manifest headers from the existing hls.js request", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const onEffectiveProfile = vi.fn();
    const onActualStart = vi.fn();
    await renderHls({
      requestedStartSec: 590,
      onEffectiveProfile,
      onActualStart,
    });

    act(() => {
      fakeHlsInstances[0].trigger("hlsManifestLoaded", {
        networkDetails: {
          getResponseHeader: (name: string) => {
            if (name === "X-Igloo-Effective-Profile") return "remux";
            if (name === "X-Igloo-Actual-Start") return "581";
            return null;
          },
        },
      });
    });

    expect(fetchMock).not.toHaveBeenCalled();
    expect(onEffectiveProfile).toHaveBeenCalledWith("remux");
    expect(onActualStart).toHaveBeenCalledWith(581);
  });

  it("rebuilds at most once when an earlier actual start changes the local offset", async () => {
    function ActualStartHarness() {
      const videoRef = useRef<HTMLVideoElement>(null);
      const [actualStartSec, setActualStartSec] = useState(590);
      return (
        <VideoPlayer
          videoRef={videoRef}
          src={hlsSrc}
          isHlsSource
          title="Test Movie"
          onError={vi.fn()}
          startSec={600 - actualStartSec}
          requestedStartSec={590}
          onActualStart={(nextStartSec) => {
            setActualStartSec((previous) =>
              previous === nextStartSec ? previous : nextStartSec,
            );
          }}
        />
      );
    }

    render(<ActualStartHarness />);
    await waitFor(() => {
      expect(fakeHlsInstances).toHaveLength(1);
    });

    act(() => {
      fakeHlsInstances[0].trigger("hlsManifestLoaded", {
        networkDetails: {
          getResponseHeader: (name: string) =>
            name === "X-Igloo-Actual-Start" ? "581" : null,
        },
      });
    });
    await waitFor(() => {
      expect(fakeHlsInstances).toHaveLength(2);
    });

    act(() => {
      fakeHlsInstances[1].trigger("hlsManifestLoaded", {
        networkDetails: {
          getResponseHeader: (name: string) =>
            name === "X-Igloo-Actual-Start" ? "581" : null,
        },
      });
    });
    await act(async () => {});
    expect(fakeHlsInstances).toHaveLength(2);
  });
});

function nativeManifestResponse(
  status: number,
  headers: Record<string, string> = {},
) {
  const cancel = vi.fn().mockResolvedValue(undefined);
  const response = {
    status,
    ok: status >= 200 && status < 300,
    headers: new Headers(headers),
    body: { cancel },
  } as unknown as Response;
  return { response, cancel };
}

describe("VideoPlayer native HLS manifest preflight", () => {
  const firstSrc = "/api/movies/1/hls/remux/playlist.m3u8?start=590";
  const secondSrc =
    "/api/movies/1/hls/remux/playlist.m3u8?start=700&reload=1";

  it("uses one authenticated preflight, releases its body, and reports metadata", async () => {
    nativeHlsSupport.supported = true;
    const { response, cancel } = nativeManifestResponse(200, {
      "X-Igloo-Effective-Profile": "remux",
      "X-Igloo-Actual-Start": "581",
    });
    const fetchMock = vi.fn().mockResolvedValue(response);
    vi.stubGlobal("fetch", fetchMock);
    const onEffectiveProfile = vi.fn();
    const onActualStart = vi.fn();

    const { video } = renderPlayer({
      src: firstSrc,
      isHlsSource: true,
      requestedStartSec: 590,
      onEffectiveProfile,
      onActualStart,
    });

    await waitFor(() => {
      expect(video).toHaveAttribute("src", firstSrc);
    });
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledWith(firstSrc, {
      credentials: "include",
      signal: expect.any(AbortSignal),
    });
    expect(cancel).toHaveBeenCalledOnce();
    expect(onEffectiveProfile).toHaveBeenCalledWith("remux");
    expect(onActualStart).toHaveBeenCalledWith(581);
  });

  it("issues only one preflight through the Strict Mode effect probe", async () => {
    nativeHlsSupport.supported = true;
    const { response } = nativeManifestResponse(200, {
      "X-Igloo-Actual-Start": "581",
    });
    const fetchMock = vi.fn().mockResolvedValue(response);
    vi.stubGlobal("fetch", fetchMock);
    const videoRef = createRef<HTMLVideoElement>();

    render(
      <StrictMode>
        <VideoPlayer
          videoRef={videoRef}
          src={firstSrc}
          isHlsSource
          title="Test Movie"
          onError={vi.fn()}
          requestedStartSec={590}
          onActualStart={vi.fn()}
        />
      </StrictMode>,
    );

    await waitFor(() => {
      expect(videoRef.current).toHaveAttribute("src", firstSrc);
    });
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("skips the preflight when native HLS has no manifest consumer", () => {
    nativeHlsSupport.supported = true;
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    const { video } = renderPlayer({
      src: firstSrc,
      isHlsSource: true,
    });

    expect(video).toHaveAttribute("src", firstSrc);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("aborts a stale preflight and never assigns its obsolete source", async () => {
    nativeHlsSupport.supported = true;
    let resolveFirst!: (response: Response) => void;
    let resolveSecond!: (response: Response) => void;
    const firstPromise = new Promise<Response>((resolve) => {
      resolveFirst = resolve;
    });
    const secondPromise = new Promise<Response>((resolve) => {
      resolveSecond = resolve;
    });
    const fetchMock = vi
      .fn()
      .mockReturnValueOnce(firstPromise)
      .mockReturnValueOnce(secondPromise);
    vi.stubGlobal("fetch", fetchMock);
    const videoRef = createRef<HTMLVideoElement>();
    const baseProps = {
      videoRef,
      isHlsSource: true,
      title: "Test Movie",
      onError: vi.fn(),
      requestedStartSec: 590,
      onActualStart: vi.fn(),
    };
    const view = render(<VideoPlayer {...baseProps} src={firstSrc} />);
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledOnce();
    });
    const firstSignal = fetchMock.mock.calls[0][1]?.signal as AbortSignal;

    view.rerender(
      <VideoPlayer
        {...baseProps}
        src={secondSrc}
        requestedStartSec={700}
      />,
    );
    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledTimes(2);
    });
    expect(firstSignal.aborted).toBe(true);

    await act(async () => {
      resolveFirst(nativeManifestResponse(200).response);
      resolveSecond(nativeManifestResponse(200).response);
    });
    await waitFor(() => {
      expect(videoRef.current).toHaveAttribute("src", secondSrc);
    });
    expect(videoRef.current).not.toHaveAttribute("src", firstSrc);
  });

  it("honors a capacity 503 without immediately assigning the failing source", async () => {
    nativeHlsSupport.supported = true;
    const { response, cancel } = nativeManifestResponse(503, {
      "Retry-After": "7",
    });
    const fetchMock = vi.fn().mockResolvedValue(response);
    vi.stubGlobal("fetch", fetchMock);
    const onCapacityBusy = vi.fn();

    const { video } = renderPlayer({
      src: firstSrc,
      isHlsSource: true,
      onCapacityBusy,
    });

    await waitFor(() => {
      expect(onCapacityBusy).toHaveBeenCalledWith(7);
    });
    expect(cancel).toHaveBeenCalledOnce();
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(video).not.toHaveAttribute("src");
  });

  it("routes a manifest 404 to onSessionLost instead of the native player", async () => {
    nativeHlsSupport.supported = true;
    const { response, cancel } = nativeManifestResponse(404);
    const fetchMock = vi.fn().mockResolvedValue(response);
    vi.stubGlobal("fetch", fetchMock);
    const onSessionLost = vi.fn();

    const { video } = renderPlayer({
      src: firstSrc,
      isHlsSource: true,
      onSessionLost,
    });

    await waitFor(() => {
      expect(onSessionLost).toHaveBeenCalledWith(0);
    });
    // onSessionLost alone must enable the preflight; without it Safari would
    // hand the dead session's 404 to the native player as a generic error.
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(cancel).toHaveBeenCalledOnce();
    expect(video).not.toHaveAttribute("src");
  });

  it("assigns a 404 source normally when no onSessionLost consumer exists", async () => {
    nativeHlsSupport.supported = true;
    const { response } = nativeManifestResponse(404);
    const fetchMock = vi.fn().mockResolvedValue(response);
    vi.stubGlobal("fetch", fetchMock);

    const { video } = renderPlayer({
      src: firstSrc,
      isHlsSource: true,
      onActualStart: vi.fn(),
      requestedStartSec: 590,
    });

    await waitFor(() => {
      expect(video).toHaveAttribute("src", firstSrc);
    });
    expect(fetchMock).toHaveBeenCalledOnce();
  });

  it("falls back to native loading when the preflight stalls past the timeout", async () => {
    vi.useFakeTimers();
    nativeHlsSupport.supported = true;
    const fetchMock = vi.fn(
      (_url: string, options: { signal: AbortSignal }) =>
        new Promise<Response>((_resolve, reject) => {
          options.signal.addEventListener("abort", () =>
            reject(new DOMException("aborted", "AbortError")),
          );
        }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const onSessionLost = vi.fn();

    const { video } = renderPlayer({
      src: firstSrc,
      isHlsSource: true,
      onSessionLost,
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(HLS_JS_LOAD_TIMEOUT_MS - 1);
    });
    expect(video).not.toHaveAttribute("src");

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(video).toHaveAttribute("src", firstSrc);
    expect(onSessionLost).not.toHaveBeenCalled();
  });

  it("lets native HLS load normally after an unexpected preflight failure", async () => {
    nativeHlsSupport.supported = true;
    const fetchMock = vi.fn().mockRejectedValue(new Error("network down"));
    vi.stubGlobal("fetch", fetchMock);

    const { video } = renderPlayer({
      src: firstSrc,
      isHlsSource: true,
      onActualStart: vi.fn(),
      requestedStartSec: 590,
    });

    await waitFor(() => {
      expect(video).toHaveAttribute("src", firstSrc);
    });
    expect(fetchMock).toHaveBeenCalledOnce();
  });
});

// A picture-in-picture or native fullscreen scrubber writes currentTime
// directly, and a drag reads currentTime as the previous pending target, so
// the page's rebase rule needs every seek measured from where playback last
// came to rest. Otherwise a far seek waits minutes for a segment the encoder
// has not reached. A run of seeks is reported once it settles: reporting each
// step of a drag would rebase, and start a transcode, once per 120 s of travel.
describe("VideoPlayer HLS seek reporting", () => {
  const hlsSrc =
    "/api/movies/1/hls/1080p_8mbps/playlist.m3u8?playback_session=uuid&start=990";

  function setPlayhead(video: HTMLElement, currentTime: number, seeking: boolean) {
    Object.defineProperty(video, "currentTime", {
      configurable: true,
      writable: true,
      value: currentTime,
    });
    Object.defineProperty(video, "seeking", {
      configurable: true,
      value: seeking,
    });
  }

  function playTo(video: HTMLElement, time: number) {
    setPlayhead(video, time, false);
    fireEvent(video, new Event("timeupdate"));
  }

  function seekTo(video: HTMLElement, time: number) {
    setPlayhead(video, time, true);
    fireEvent(video, new Event("seeking"));
  }

  function settleAt(video: HTMLElement, time: number) {
    setPlayhead(video, time, false);
    fireEvent(video, new Event("seeked"));
  }

  /** Lets the element go quiet for long enough that the pending run is reported. */
  async function settleSeeks() {
    await act(async () => {
      await vi.advanceTimersByTimeAsync(HLS_SEEK_SETTLE_MS);
    });
  }

  async function renderSeekingPlayer(
    props: Partial<React.ComponentProps<typeof VideoPlayer>> = {},
  ) {
    vi.useFakeTimers();
    const onHlsSeeking = vi.fn();
    const result = renderPlayer({
      src: hlsSrc,
      isHlsSource: true,
      startSec: 10,
      onHlsSeeking,
      ...props,
    });
    await act(async () => {});
    return { ...result, onHlsSeeking };
  }

  it("reports a seek before playback settles from the start the source was built for", async () => {
    const { video, onHlsSeeking } = await renderSeekingPlayer();

    seekTo(video, 4000);
    await settleSeeks();

    expect(onHlsSeeking).toHaveBeenCalledWith(10, 4000);
  });

  it("reports a seek from the playhead playback last settled at", async () => {
    const { video, onHlsSeeking } = await renderSeekingPlayer();

    playTo(video, 42);
    seekTo(video, 4000);
    await settleSeeks();

    expect(onHlsSeeking).toHaveBeenCalledWith(42, 4000);
  });

  it("waits for the element to go quiet before reporting", async () => {
    const { video, onHlsSeeking } = await renderSeekingPlayer();

    seekTo(video, 4000);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(HLS_SEEK_SETTLE_MS - 1);
    });

    expect(onHlsSeeking).not.toHaveBeenCalled();
  });

  it("reports a run of seeks once, from the resting point to where it ended", async () => {
    const { video, onHlsSeeking } = await renderSeekingPlayer();

    playTo(video, 42);
    seekTo(video, 100);
    // timeupdate also fires while a seek is pending; it is not a resting point.
    fireEvent(video, new Event("timeupdate"));
    await act(async () => {
      await vi.advanceTimersByTimeAsync(HLS_SEEK_SETTLE_MS - 1);
    });
    seekTo(video, 160);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(HLS_SEEK_SETTLE_MS - 1);
    });
    seekTo(video, 220);
    await settleSeeks();

    expect(onHlsSeeking).toHaveBeenCalledOnce();
    expect(onHlsSeeking).toHaveBeenCalledWith(42, 220);
  });

  it("moves the resting point once a seek completes", async () => {
    const { video, onHlsSeeking } = await renderSeekingPlayer();

    playTo(video, 42);
    seekTo(video, 100);
    settleAt(video, 100);
    seekTo(video, 160);
    await settleSeeks();

    expect(onHlsSeeking).toHaveBeenLastCalledWith(100, 160);
  });

  it("measures a new source from its own start, not the previous source's playhead", async () => {
    vi.useFakeTimers();
    const onHlsSeeking = vi.fn();
    const videoRef = createRef<HTMLVideoElement>();
    const baseProps = {
      videoRef,
      isHlsSource: true,
      title: "Test Movie",
      onError: vi.fn(),
      onHlsSeeking,
    };
    const { rerender } = render(
      <VideoPlayer {...baseProps} src={hlsSrc} startSec={10} />,
    );
    await act(async () => {});
    const video = screen.getByLabelText("Video player for Test Movie");
    playTo(video, 2500);

    rerender(
      <VideoPlayer
        {...baseProps}
        src={hlsSrc.replace("start=990", "start=3990")}
        startSec={12}
      />,
    );
    await act(async () => {});
    seekTo(video, 15);
    await settleSeeks();

    expect(onHlsSeeking).toHaveBeenLastCalledWith(12, 15);
  });

  it("drops a seek the source change already answered", async () => {
    vi.useFakeTimers();
    const onHlsSeeking = vi.fn();
    const videoRef = createRef<HTMLVideoElement>();
    const baseProps = {
      videoRef,
      isHlsSource: true,
      title: "Test Movie",
      onError: vi.fn(),
      onHlsSeeking,
    };
    const { rerender } = render(
      <VideoPlayer {...baseProps} src={hlsSrc} startSec={10} />,
    );
    await act(async () => {});
    const video = screen.getByLabelText("Video player for Test Movie");
    seekTo(video, 4000);

    rerender(
      <VideoPlayer
        {...baseProps}
        src={hlsSrc.replace("start=990", "start=3990")}
        startSec={10}
      />,
    );
    await act(async () => {});
    await settleSeeks();

    expect(onHlsSeeking).not.toHaveBeenCalled();
  });

  it("reports seeks on native HLS too", async () => {
    nativeHlsSupport.supported = true;
    const { video, onHlsSeeking } = await renderSeekingPlayer();

    seekTo(video, 4000);
    await settleSeeks();

    expect(onHlsSeeking).toHaveBeenCalledWith(10, 4000);
  });

  it("never reports a direct-play seek", async () => {
    vi.useFakeTimers();
    const onHlsSeeking = vi.fn();
    const { video } = renderPlayer({ onHlsSeeking });

    seekTo(video, 4000);
    await settleSeeks();

    expect(onHlsSeeking).not.toHaveBeenCalled();
  });
});
