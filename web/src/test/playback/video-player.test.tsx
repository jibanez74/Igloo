import { createRef, StrictMode, useRef, useState } from "react";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from "vitest";
import VideoPlayer from "@/components/playback/VideoPlayer";
import {
  HLS_CAPACITY_RETRY_FALLBACK_SEC,
  MOVIE_BUFFERING_SPINNER_DELAY_MS,
} from "@/lib/constants";

type FakeHlsListener = (event: string, data: unknown) => void;

const fakeHlsInstances = vi.hoisted(() => [] as FakeHlsInstance[]);
const fakeHlsSupport = vi.hoisted(() => ({ supported: true }));
const nativeHlsSupport = vi.hoisted(() => ({ supported: false }));

type FakeHlsInstance = {
  listeners: Map<string, FakeHlsListener[]>;
  trigger: (event: string, data: unknown) => void;
  destroyed: boolean;
};

vi.mock("@/lib/playback", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/playback")>();
  return {
    ...actual,
    get supportsNativeHLS() {
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
    };
    static ErrorDetails = {
      FRAG_LOAD_ERROR: "fragLoadError",
      MANIFEST_LOAD_ERROR: "manifestLoadError",
      LEVEL_LOAD_ERROR: "levelLoadError",
    };

    listeners = new Map<string, FakeHlsListener[]>();
    destroyed = false;

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
      for (const listener of this.listeners.get(event) ?? []) {
        listener(event, data);
      }
    }

    loadSource() {}
    attachMedia() {}
    recoverMediaError() {}
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
  vi.unstubAllGlobals();
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
