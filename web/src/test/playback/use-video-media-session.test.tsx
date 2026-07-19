import { cleanup, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useVideoMediaSession } from "@/hooks/useVideoMediaSession";

const originalMediaSessionDescriptor = Object.getOwnPropertyDescriptor(
  navigator,
  "mediaSession",
);
const originalMediaMetadataDescriptor = Object.getOwnPropertyDescriptor(
  globalThis,
  "MediaMetadata",
);

type MockMediaSession = {
  metadata: unknown;
  playbackState: string;
  setActionHandler: ReturnType<typeof vi.fn>;
  setPositionState: ReturnType<typeof vi.fn>;
};

function mockMediaSession(): MockMediaSession {
  const mediaSession: MockMediaSession = {
    metadata: null,
    playbackState: "none",
    setActionHandler: vi.fn(),
    setPositionState: vi.fn(),
  };

  Object.defineProperty(navigator, "mediaSession", {
    configurable: true,
    value: mediaSession,
  });

  return mediaSession;
}

function restoreProperty(
  object: object,
  property: string,
  descriptor: PropertyDescriptor | undefined,
) {
  if (descriptor) {
    Object.defineProperty(object, property, descriptor);
    return;
  }

  Reflect.deleteProperty(object, property);
}

function capturedActionHandler(
  mediaSession: MockMediaSession,
  action: string,
) {
  const call = mediaSession.setActionHandler.mock.calls
    .filter(([name]) => name === action)
    .at(-1);
  return call?.[1] as ((details: MediaSessionActionDetails) => void) | null;
}

type HarnessProps = {
  currentTime: number;
  duration: number;
  playing: boolean;
  enabled?: boolean;
  videoRef: React.RefObject<HTMLVideoElement | null>;
  onPlay?: () => void;
  onPause?: () => void;
  onSeek?: (time: number) => void;
};

function Harness({
  currentTime,
  duration,
  playing,
  enabled = true,
  videoRef,
  onPlay = () => {},
  onPause = () => {},
  onSeek = () => {},
}: HarnessProps) {
  useVideoMediaSession({
    videoRef,
    title: "Test Movie",
    artist: "Igloo",
    artworkUrl: "/artwork.jpg",
    currentTime,
    duration,
    playing,
    enabled,
    onPlay,
    onPause,
    onSeek,
  });
  return null;
}

function makeVideoRef() {
  return { current: document.createElement("video") };
}

describe("useVideoMediaSession", () => {
  let mediaSession: MockMediaSession;

  beforeEach(() => {
    mediaSession = mockMediaSession();
    Object.defineProperty(globalThis, "MediaMetadata", {
      configurable: true,
      value: vi.fn(function MediaMetadata(
        this: Record<string, unknown>,
        metadata: Record<string, unknown>,
      ) {
        Object.assign(this, metadata);
      }),
    });
  });

  afterEach(() => {
    // Unmount before removing the mediaSession mock so unmount effects can
    // still reach it (the global setup's cleanup would run too late).
    cleanup();
    restoreProperty(navigator, "mediaSession", originalMediaSessionDescriptor);
    restoreProperty(
      globalThis,
      "MediaMetadata",
      originalMediaMetadataDescriptor,
    );
  });

  it("publishes metadata with absolute artwork and clears it on unmount", () => {
    const { unmount } = render(
      <Harness
        currentTime={0}
        duration={600}
        playing={false}
        videoRef={makeVideoRef()}
      />,
    );

    expect(mediaSession.metadata).toMatchObject({
      title: "Test Movie",
      artist: "Igloo",
      artwork: [{ src: `${window.location.origin}/artwork.jpg` }],
    });

    unmount();
    expect(mediaSession.metadata).toBeNull();
  });

  it("mirrors the playing state onto playbackState", () => {
    const videoRef = makeVideoRef();
    const { rerender } = render(
      <Harness
        currentTime={0}
        duration={600}
        playing={false}
        videoRef={videoRef}
      />,
    );
    expect(mediaSession.playbackState).toBe("paused");

    rerender(
      <Harness currentTime={0} duration={600} playing videoRef={videoRef} />,
    );
    expect(mediaSession.playbackState).toBe("playing");
  });

  it("registers action handlers, clamps seeks, and unregisters on unmount", () => {
    const onSeek = vi.fn();
    const { unmount } = render(
      <Harness
        currentTime={30}
        duration={600}
        playing
        videoRef={makeVideoRef()}
        onSeek={onSeek}
      />,
    );

    for (const action of [
      "play",
      "pause",
      "seekbackward",
      "seekforward",
      "seekto",
    ]) {
      expect(capturedActionHandler(mediaSession, action)).toBeTypeOf(
        "function",
      );
    }

    capturedActionHandler(mediaSession, "seekbackward")?.({
      action: "seekbackward",
      seekOffset: 50,
    });
    expect(onSeek).toHaveBeenLastCalledWith(0);

    capturedActionHandler(mediaSession, "seekforward")?.({
      action: "seekforward",
      seekOffset: 1000,
    });
    expect(onSeek).toHaveBeenLastCalledWith(600);

    capturedActionHandler(mediaSession, "seekto")?.({
      action: "seekto",
      seekTime: 120,
    });
    expect(onSeek).toHaveBeenLastCalledWith(120);

    unmount();
    expect(capturedActionHandler(mediaSession, "seekto")).toBeNull();
  });

  it("skips position updates below the drift threshold", () => {
    const videoRef = makeVideoRef();
    const { rerender } = render(
      <Harness
        currentTime={10}
        duration={600}
        playing
        videoRef={videoRef}
      />,
    );
    expect(mediaSession.setPositionState).toHaveBeenCalledTimes(1);
    expect(mediaSession.setPositionState).toHaveBeenLastCalledWith({
      duration: 600,
      playbackRate: 1,
      position: 10,
    });

    // Under the 5s threshold relative to the last *reported* position.
    rerender(
      <Harness currentTime={12} duration={600} playing videoRef={videoRef} />,
    );
    rerender(
      <Harness currentTime={14} duration={600} playing videoRef={videoRef} />,
    );
    expect(mediaSession.setPositionState).toHaveBeenCalledTimes(1);

    // 16 - 10 >= 5: reports again.
    rerender(
      <Harness currentTime={16} duration={600} playing videoRef={videoRef} />,
    );
    expect(mediaSession.setPositionState).toHaveBeenCalledTimes(2);
    expect(mediaSession.setPositionState).toHaveBeenLastCalledWith({
      duration: 600,
      playbackRate: 1,
      position: 16,
    });
  });

  it("reports promptly on play/pause and duration changes", () => {
    const videoRef = makeVideoRef();
    const { rerender } = render(
      <Harness
        currentTime={10}
        duration={600}
        playing
        videoRef={videoRef}
      />,
    );
    expect(mediaSession.setPositionState).toHaveBeenCalledTimes(1);

    rerender(
      <Harness
        currentTime={11}
        duration={600}
        playing={false}
        videoRef={videoRef}
      />,
    );
    expect(mediaSession.setPositionState).toHaveBeenCalledTimes(2);

    rerender(
      <Harness
        currentTime={11}
        duration={720}
        playing={false}
        videoRef={videoRef}
      />,
    );
    expect(mediaSession.setPositionState).toHaveBeenCalledTimes(3);
  });

  it("never reports a position without a valid duration", () => {
    const videoRef = makeVideoRef();
    const { rerender } = render(
      <Harness currentTime={10} duration={0} playing videoRef={videoRef} />,
    );
    rerender(
      <Harness currentTime={20} duration={0} playing videoRef={videoRef} />,
    );
    expect(mediaSession.setPositionState).not.toHaveBeenCalled();
  });
});
