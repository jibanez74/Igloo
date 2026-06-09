import { createRef } from "react";
import { act, cleanup, render, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import AudioPlayer from "@/components/AudioPlayer";
import type { TrackType } from "@/types";

const originalMediaMetadataDescriptor = Object.getOwnPropertyDescriptor(
  globalThis,
  "MediaMetadata",
);
const originalNavigatorMediaSessionDescriptor = Object.getOwnPropertyDescriptor(
  navigator,
  "mediaSession",
);
const originalLoad = HTMLMediaElement.prototype.load;
const originalPlay = HTMLMediaElement.prototype.play;
const originalPause = HTMLMediaElement.prototype.pause;

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

function track(): TrackType {
  return {
    id: 42,
    title: "Alabaster",
    sort_title: "Alabaster",
    file_path: "/music/alabaster.flac",
    file_name: "alabaster.flac",
    container: "flac",
    mime_type: "audio/flac",
    codec: "flac",
    size: 1024,
    track_index: 1,
    duration: 180,
    disc: 1,
    channels: "2",
    channel_layout: "stereo",
    bit_rate: 900000,
    profile: "",
    release_date: nullableString("2026-01-01"),
    year: nullableInt64(2026),
    composer: nullableString(),
    copyright: nullableString(),
    language: nullableString("en"),
    album_id: nullableInt64(7),
    musician_id: nullableInt64(8),
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function setAudioNumber(
  audio: HTMLAudioElement,
  property: "currentTime" | "duration" | "playbackRate",
  value: number,
) {
  Object.defineProperty(audio, property, {
    configurable: true,
    value,
  });
}

function renderAudioPlayer() {
  const audioRef = createRef<HTMLAudioElement>();
  const currentTrack = track();
  const view = render(
    <AudioPlayer
      track={currentTrack}
      tracks={[currentTrack]}
      albumCover="/covers/7.jpg"
      albumTitle="Blue Record"
      musicianName="The Band"
      onTrackChange={vi.fn()}
      audioRef={audioRef}
      isPlaying={false}
      onPlayStateChange={vi.fn()}
      isExpanded={false}
      onMinimize={vi.fn()}
      onExpand={vi.fn()}
    />,
  );

  const audio = view.container.querySelector("audio");
  if (!audio) {
    throw new Error("Audio element was not rendered");
  }

  return {
    ...view,
    audio,
  };
}

function mockMediaSession() {
  const mediaSession = {
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

describe("AudioPlayer Media Session", () => {
  beforeEach(() => {
    HTMLMediaElement.prototype.load = vi.fn();
    HTMLMediaElement.prototype.play = vi.fn().mockResolvedValue(undefined);
    HTMLMediaElement.prototype.pause = vi.fn();

    Object.defineProperty(globalThis, "MediaMetadata", {
      configurable: true,
      value: vi.fn(function MediaMetadata(metadata) {
        return metadata;
      }),
    });
  });

  afterEach(() => {
    cleanup();

    restoreProperty(
      navigator,
      "mediaSession",
      originalNavigatorMediaSessionDescriptor,
    );
    restoreProperty(
      globalThis,
      "MediaMetadata",
      originalMediaMetadataDescriptor,
    );

    HTMLMediaElement.prototype.load = originalLoad;
    HTMLMediaElement.prototype.play = originalPlay;
    HTMLMediaElement.prototype.pause = originalPause;
  });

  it("does not sync position when audio duration is NaN", async () => {
    const mediaSession = mockMediaSession();
    const { audio } = renderAudioPlayer();

    await waitFor(() => {
      expect(HTMLMediaElement.prototype.load).toHaveBeenCalled();
    });

    setAudioNumber(audio, "duration", Number.NaN);
    setAudioNumber(audio, "currentTime", 12);

    expect(() => {
      act(() => {
        audio.dispatchEvent(new Event("durationchange"));
        audio.dispatchEvent(new Event("timeupdate"));
      });
    }).not.toThrow();

    expect(mediaSession.setPositionState).not.toHaveBeenCalled();
  });

  it("sends finite, clamped position state values", async () => {
    const mediaSession = mockMediaSession();
    const { audio } = renderAudioPlayer();

    await waitFor(() => {
      expect(HTMLMediaElement.prototype.load).toHaveBeenCalled();
    });

    setAudioNumber(audio, "duration", 120);
    setAudioNumber(audio, "currentTime", Number.NaN);
    setAudioNumber(audio, "playbackRate", Number.NaN);

    act(() => {
      audio.dispatchEvent(new Event("timeupdate"));
    });

    expect(mediaSession.setPositionState).toHaveBeenLastCalledWith({
      duration: 120,
      playbackRate: 1,
      position: 0,
    });

    setAudioNumber(audio, "currentTime", 999);
    setAudioNumber(audio, "playbackRate", 1.25);

    act(() => {
      audio.dispatchEvent(new Event("timeupdate"));
    });

    expect(mediaSession.setPositionState).toHaveBeenLastCalledWith({
      duration: 120,
      playbackRate: 1.25,
      position: 120,
    });
  });

  it("does not throw when MediaMetadata is unavailable", () => {
    mockMediaSession();
    Object.defineProperty(globalThis, "MediaMetadata", {
      configurable: true,
      value: undefined,
    });

    expect(() => renderAudioPlayer()).not.toThrow();
  });
});
