import { cleanup, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAudioMediaSession } from "@/hooks/useAudioMediaSession";
import { useVideoMediaSession } from "@/hooks/useVideoMediaSession";
import { MEDIA_SESSION_ACTIONS } from "@/lib/media-session";

// Music and video share navigator.mediaSession. These mount both real hooks,
// the way the app does (the music player stays mounted under every page), and
// read the session as the OS would: the handler last registered per action.

const originalMediaSession = Object.getOwnPropertyDescriptor(navigator, "mediaSession");
const originalMediaMetadata = Object.getOwnPropertyDescriptor(globalThis, "MediaMetadata");

type Handler = ((details: MediaSessionActionDetails) => void) | null;

function fakeMediaSession() {
  const handlers = new Map<string, Handler>();
  const session = {
    metadata: null as { title?: string } | null,
    playbackState: "none",
    setActionHandler: vi.fn((action: string, handler: Handler) => {
      handlers.set(action, handler);
    }),
    setPositionState: vi.fn(),
  };
  Object.defineProperty(navigator, "mediaSession", { configurable: true, value: session });
  return { session, handlers };
}

function restore(object: object, property: string, descriptor?: PropertyDescriptor) {
  if (descriptor) Object.defineProperty(object, property, descriptor);
  else Reflect.deleteProperty(object, property);
}

type MusicProps = { isPlaying: boolean; trackId?: number | null };

const musicAudio = { current: document.createElement("audio") };

function Music({ isPlaying, trackId = 1 }: MusicProps) {
  useAudioMediaSession({
    audioRef: musicAudio,
    currentTrackId: trackId,
    trackTitle: trackId === null ? null : "Song",
    albumTitle: "Album",
    albumCover: null,
    musicianName: "Band",
    isPlaying,
    onStop: () => {},
    onPrevious: () => {},
    onNext: () => {},
    onSeekBy: () => {},
  });
  return null;
}

const movieVideo = { current: document.createElement("video") };

function Video({ enabled = true, title = "Movie" }: { enabled?: boolean; title?: string }) {
  useVideoMediaSession({
    videoRef: movieVideo,
    title,
    currentTime: 0,
    duration: 100,
    playing: true,
    enabled,
    onPlay: () => {},
    onPause: () => {},
    onSeek: () => {},
  });
  return null;
}

function App({
  music,
  video,
}: {
  music: MusicProps;
  video?: { enabled?: boolean; title?: string };
}) {
  return (
    <>
      <Music {...music} />
      {video ? <Video {...video} /> : null}
    </>
  );
}

const MUSIC_ONLY = ["stop", "previoustrack", "nexttrack"];

describe("Media Session ownership between music and video", () => {
  let fake: ReturnType<typeof fakeMediaSession>;

  beforeEach(() => {
    fake = fakeMediaSession();
    Object.defineProperty(globalThis, "MediaMetadata", {
      configurable: true,
      value: vi.fn(function MediaMetadata(this: object, init: object) {
        Object.assign(this, init);
      }),
    });
  });

  afterEach(() => {
    cleanup();
    restore(navigator, "mediaSession", originalMediaSession);
    restore(globalThis, "MediaMetadata", originalMediaMetadata);
  });

  const registered = () =>
    MEDIA_SESSION_ACTIONS.filter((action) => fake.handlers.get(action));

  it("gives a video the whole session, then hands music its controls back", () => {
    const { rerender } = render(<App music={{ isPlaying: true }} />);
    expect(fake.session.metadata?.title).toBe("Song");
    expect(registered()).toEqual([...MEDIA_SESSION_ACTIONS]);

    // The video page mounts and claims the session while music still reports
    // playing: pause() is synchronous, its pause event arrives a task later.
    rerender(<App music={{ isPlaying: true }} video={{}} />);
    expect(fake.session.metadata?.title).toBe("Movie");

    // Then music's pause lands: none of music's previous/next/stop may stay
    // wired to the OS buttons, and music must not take the session back.
    rerender(<App music={{ isPlaying: false }} video={{}} />);
    expect(fake.session.metadata?.title).toBe("Movie");
    for (const action of MUSIC_ONLY) {
      expect(fake.handlers.get(action)).toBeNull();
    }
    expect(registered()).toEqual(["play", "pause", "seekbackward", "seekforward", "seekto"]);

    // Leaving the video, with the same track still paused: music's metadata
    // and every action come back without the track changing.
    rerender(<App music={{ isPlaying: false }} />);
    expect(fake.session.metadata?.title).toBe("Song");
    expect(registered()).toEqual([...MEDIA_SESSION_ACTIONS]);
    expect(fake.session.playbackState).toBe("paused");

    rerender(<App music={{ isPlaying: true }} />);
    expect(fake.session.playbackState).toBe("playing");
    expect(registered()).toEqual([...MEDIA_SESSION_ACTIONS]);
  });

  it("drops the video's timeline when it lets go, and publishes the paused track's", () => {
    Object.defineProperty(musicAudio.current, "duration", { configurable: true, value: 200 });
    musicAudio.current.currentTime = 42;
    try {
      const { rerender } = render(<App music={{ isPlaying: false }} video={{}} />);
      expect(fake.session.setPositionState).toHaveBeenLastCalledWith(
        expect.objectContaining({ duration: 100 }),
      );

      fake.session.setPositionState.mockClear();
      rerender(<App music={{ isPlaying: false }} />);

      // Cleared first (no argument), then the song's own timeline.
      expect(fake.session.setPositionState.mock.calls[0]).toEqual([]);
      expect(fake.session.setPositionState).toHaveBeenLastCalledWith(
        expect.objectContaining({ duration: 200, position: 42 }),
      );
    } finally {
      Reflect.deleteProperty(musicAudio.current, "duration");
      musicAudio.current.currentTime = 0;
    }
  });

  it("keeps a video's play state and controls when only its title changes", () => {
    const { rerender } = render(<App music={{ isPlaying: false }} video={{ title: "Episode 1" }} />);
    expect(fake.session.playbackState).toBe("playing");

    rerender(<App music={{ isPlaying: false }} video={{ title: "Episode 2" }} />);

    expect(fake.session.metadata?.title).toBe("Episode 2");
    expect(fake.session.playbackState).toBe("playing");
    expect(registered()).toEqual(["play", "pause", "seekbackward", "seekforward", "seekto"]);
  });

  it("does not let music take the session from a video while paused", () => {
    const { rerender } = render(<App music={{ isPlaying: false }} video={{}} />);
    expect(fake.session.metadata?.title).toBe("Movie");

    // A track change while paused (the queue advanced) must not steal it.
    rerender(<App music={{ isPlaying: false, trackId: 2 }} video={{}} />);
    expect(fake.session.metadata?.title).toBe("Movie");
  });

  it("lets music resumed during a video take the session, and the video's cleanup leave it", () => {
    const { rerender } = render(<App music={{ isPlaying: false }} video={{}} />);
    expect(fake.session.metadata?.title).toBe("Movie");

    // The reader plays music from the mini player over the video page.
    rerender(<App music={{ isPlaying: true }} video={{}} />);
    expect(fake.session.metadata?.title).toBe("Song");

    // The video's cleanup runs after music took over: it must not clear it.
    rerender(<App music={{ isPlaying: true }} />);
    expect(fake.session.metadata?.title).toBe("Song");
    expect(registered()).toEqual([...MEDIA_SESSION_ACTIONS]);
    expect(fake.session.playbackState).toBe("playing");
  });

  // Music resumed while a room was still loading keeps playing as the room's
  // video claims the controls and later lets them go: nothing about the track
  // or its play state changes when the video leaves, yet music must get them.
  it("gives playing music the controls back when a video it did not pause leaves", () => {
    const { rerender } = render(<App music={{ isPlaying: true }} />);

    rerender(<App music={{ isPlaying: true }} video={{}} />);
    expect(fake.session.metadata?.title).toBe("Movie");

    rerender(<App music={{ isPlaying: true }} />);
    expect(fake.session.metadata?.title).toBe("Song");
    expect(registered()).toEqual([...MEDIA_SESSION_ACTIONS]);
    expect(fake.session.playbackState).toBe("playing");
  });

  it("drops music's timeline when a video takes over", () => {
    Object.defineProperty(musicAudio.current, "duration", { configurable: true, value: 200 });
    try {
      const { rerender } = render(<App music={{ isPlaying: true }} />);
      fake.session.setPositionState.mockClear();

      rerender(<App music={{ isPlaying: true }} video={{}} />);

      // Cleared as the video claims, before it reports its own (duration 100).
      expect(fake.session.setPositionState.mock.calls[0]).toEqual([]);
      expect(fake.session.setPositionState).toHaveBeenLastCalledWith(
        expect.objectContaining({ duration: 100 }),
      );
    } finally {
      Reflect.deleteProperty(musicAudio.current, "duration");
    }
  });

  // The reader plays music over a video, then closes the music player: the
  // video is still on screen and gets its controls, play state and timeline
  // back without pressing anything.
  it("gives a video its controls back when music played over it is closed", () => {
    const { rerender } = render(<App music={{ isPlaying: false }} video={{}} />);
    rerender(<App music={{ isPlaying: true }} video={{}} />);
    expect(fake.session.metadata?.title).toBe("Song");
    fake.session.setPositionState.mockClear();

    rerender(<App music={{ isPlaying: false, trackId: null }} video={{}} />);

    expect(fake.session.metadata?.title).toBe("Movie");
    expect(registered()).toEqual(["play", "pause", "seekbackward", "seekforward", "seekto"]);
    expect(fake.session.playbackState).toBe("playing");
    expect(fake.session.setPositionState).toHaveBeenLastCalledWith(
      expect.objectContaining({ duration: 100 }),
    );
  });

  it("clears the session once music stops with no video", () => {
    const { rerender } = render(<App music={{ isPlaying: true }} />);
    rerender(<App music={{ isPlaying: false, trackId: null }} />);

    expect(fake.session.metadata).toBeNull();
    expect(registered()).toEqual([]);
    expect(fake.session.playbackState).toBe("none");
  });

  it("hands the session back to music when a playback error disables the video", () => {
    const { rerender } = render(<App music={{ isPlaying: false }} video={{}} />);
    rerender(<App music={{ isPlaying: false }} video={{ enabled: false }} />);

    expect(fake.session.metadata?.title).toBe("Song");
    expect(registered()).toEqual([...MEDIA_SESSION_ACTIONS]);
  });
});
