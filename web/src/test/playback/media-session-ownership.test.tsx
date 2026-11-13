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

function Video({ enabled = true }: { enabled?: boolean }) {
  useVideoMediaSession({
    videoRef: movieVideo,
    title: "Movie",
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

function App({ music, video }: { music: MusicProps; video?: { enabled?: boolean } }) {
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

    // The video page pauses music and claims the session: none of music's
    // previous/next/stop may stay wired to the OS buttons.
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
