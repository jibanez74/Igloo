import { createRef, useRef, useState } from "react";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import AudioPlayer from "@/components/AudioPlayer";
import {
  MOTION_MEDIA_OVERLAY_ENTER_CLASS,
  MOTION_PLAYER_CHROME_BUTTON_CLASS,
  MOTION_PLAYER_CHROME_ENTER_CLASS,
} from "@/lib/constants";
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

function renderAudioPlayer({
  isExpanded = false,
  onClose,
}: {
  isExpanded?: boolean;
  onClose?: () => void;
} = {}) {
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
      isExpanded={isExpanded}
      onMinimize={vi.fn()}
      onExpand={vi.fn()}
      onClose={onClose}
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

function AudioPlayerFocusHarness() {
  const [isExpanded, setIsExpanded] = useState(true);
  const audioRef = useRef<HTMLAudioElement | null>(null);
  const currentTrack = track();

  return (
    <>
      <button type="button">Outside player</button>
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
        isExpanded={isExpanded}
        onMinimize={() => setIsExpanded(false)}
        onExpand={() => setIsExpanded(true)}
        onClose={vi.fn()}
      />
    </>
  );
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

  it("keeps minimized audio chrome labelled and reduced-motion safe", () => {
    renderAudioPlayer({ onClose: vi.fn() });

    expect(screen.getByRole("region", { name: "Audio player" })).toHaveClass(
      ...MOTION_PLAYER_CHROME_ENTER_CLASS.split(" "),
    );
    expect(
      screen.getByRole("button", {
        name: /expand player\. now playing: alabaster by the band/i,
      }),
    ).toHaveClass(...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "));
    expect(
      screen.getByRole("button", { name: "No previous track" }),
    ).toHaveClass(...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "));
    expect(screen.getByRole("button", { name: "Play" })).toHaveClass(
      ...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "),
    );
    expect(screen.getByRole("button", { name: "No next track" })).toHaveClass(
      ...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "),
    );
    expect(
      screen.getByRole("button", { name: "Stop playback and close player" }),
    ).toHaveClass(...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "));
  });

  it("keeps expanded audio chrome labelled and reduced-motion safe", () => {
    renderAudioPlayer({ isExpanded: true, onClose: vi.fn() });

    expect(
      screen.getByRole("dialog", {
        name: "Now playing: Alabaster by The Band",
      }),
    ).toHaveClass(...MOTION_MEDIA_OVERLAY_ENTER_CLASS.split(" "));
    expect(
      screen.getByRole("button", { name: "Minimize player (Escape)" }),
    ).toHaveClass(...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "));
    expect(
      screen.getByRole("button", { name: "Stop playback and close player" }),
    ).toHaveClass(...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "));
    expect(
      screen.getByRole("button", { name: "No previous track" }),
    ).toHaveClass(...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "));
    expect(screen.getByRole("button", { name: "Play" })).toHaveClass(
      ...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "),
    );
    expect(screen.getByRole("button", { name: "No next track" })).toHaveClass(
      ...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "),
    );
    expect(screen.getByRole("button", { name: "Mute" })).toHaveClass(
      ...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "),
    );
  });

  it("focuses playback controls when the expanded player opens", async () => {
    renderAudioPlayer({ isExpanded: true, onClose: vi.fn() });

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Play" })).toHaveFocus();
    });
  });

  it("keeps tab focus inside the expanded player", async () => {
    const user = userEvent.setup();
    render(
      <>
        <button type="button">Outside player</button>
        <AudioPlayer
          track={track()}
          tracks={[track()]}
          albumCover="/covers/7.jpg"
          albumTitle="Blue Record"
          musicianName="The Band"
          onTrackChange={vi.fn()}
          audioRef={createRef<HTMLAudioElement>()}
          isPlaying={false}
          onPlayStateChange={vi.fn()}
          isExpanded
          onMinimize={vi.fn()}
          onExpand={vi.fn()}
          onClose={vi.fn()}
        />
      </>,
    );

    const dialog = screen.getByRole("dialog", {
      name: "Now playing: Alabaster by The Band",
    });
    const focusableElements = dialog.querySelectorAll<HTMLElement>(
      'button:not([disabled]), input:not([disabled])',
    );
    const lastElement = focusableElements[focusableElements.length - 1];
    lastElement.focus();

    await user.tab();

    expect(dialog).toContainElement(document.activeElement as HTMLElement);
  });

  it("restores focus to the minimized expand control after Escape", async () => {
    const user = userEvent.setup();
    render(<AudioPlayerFocusHarness />);

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Play" })).toHaveFocus();
    });

    await user.keyboard("{Escape}");

    const expandButton = await screen.findByRole("button", {
      name: /expand player\. now playing: alabaster by the band/i,
    });
    await waitFor(() => {
      expect(expandButton).toHaveFocus();
    });
  });
});
