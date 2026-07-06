import { createRef, useRef, useState, type ReactNode } from "react";
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
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

function track(overrides: Partial<TrackType> = {}): TrackType {
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
    ...overrides,
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

// setAudioNumber defines a read-only value; the restart tests need to observe
// the `audio.currentTime = 0` assignment, so this installs a writable spy.
function observeAudioCurrentTime(audio: HTMLAudioElement, initial: number) {
  let currentValue = initial;
  const set = vi.fn((next: number) => {
    currentValue = next;
  });

  Object.defineProperty(audio, "currentTime", {
    configurable: true,
    get: () => currentValue,
    set,
  });

  return set;
}

function renderAudioPlayer({
  isExpanded = false,
  onClose,
  currentTrack = track(),
  tracks = [currentTrack],
  onTrackChange = vi.fn(),
  siblings,
}: {
  isExpanded?: boolean;
  onClose?: () => void;
  currentTrack?: TrackType;
  tracks?: TrackType[];
  onTrackChange?: (nextTrack: TrackType) => void;
  siblings?: ReactNode;
} = {}) {
  const audioRef = createRef<HTMLAudioElement>();
  const view = render(
    <>
      {siblings}
      <AudioPlayer
        track={currentTrack}
        tracks={tracks}
        albumCover="/covers/7.jpg"
        albumTitle="Blue Record"
        musicianName="The Band"
        onTrackChange={onTrackChange}
        audioRef={audioRef}
        isPlaying={false}
        onPlayStateChange={vi.fn()}
        isExpanded={isExpanded}
        onMinimize={vi.fn()}
        onExpand={vi.fn()}
        onClose={onClose}
      />
    </>,
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

  it("clears media session metadata when playback stops", () => {
    const mediaSession = mockMediaSession();
    const sharedProps = {
      tracks: [track()],
      albumCover: "/covers/7.jpg",
      albumTitle: "Blue Record",
      musicianName: "The Band",
      onTrackChange: vi.fn(),
      audioRef: createRef<HTMLAudioElement>(),
      isPlaying: false,
      onPlayStateChange: vi.fn(),
      isExpanded: false,
      onMinimize: vi.fn(),
      onExpand: vi.fn(),
    };

    const { rerender } = render(<AudioPlayer track={track()} {...sharedProps} />);
    expect(mediaSession.metadata).not.toBeNull();

    rerender(<AudioPlayer track={null} {...sharedProps} />);
    expect(mediaSession.metadata).toBeNull();
  });

  it("resets the displayed position when the track changes", () => {
    mockMediaSession();
    const first = track();
    const second = track({ id: 43, title: "Basalt" });
    const sharedProps = {
      tracks: [first, second],
      albumCover: "/covers/7.jpg",
      albumTitle: "Blue Record",
      musicianName: "The Band",
      onTrackChange: vi.fn(),
      audioRef: createRef<HTMLAudioElement>(),
      isPlaying: false,
      onPlayStateChange: vi.fn(),
      isExpanded: false,
      onMinimize: vi.fn(),
      onExpand: vi.fn(),
    };

    const { rerender, container } = render(
      <AudioPlayer track={first} {...sharedProps} />,
    );

    const audio = container.querySelector("audio");
    if (!audio) {
      throw new Error("Audio element was not rendered");
    }
    setAudioNumber(audio, "duration", 120);
    setAudioNumber(audio, "currentTime", 60);
    act(() => {
      audio.dispatchEvent(new Event("durationchange"));
      audio.dispatchEvent(new Event("timeupdate"));
    });

    for (const slider of screen.getAllByRole("slider", {
      name: "Seek through track",
    })) {
      expect((slider as HTMLInputElement).value).toBe("60");
    }

    // The new track's media events have not fired yet; the old position and
    // duration must not linger on the progress bar.
    rerender(<AudioPlayer track={second} {...sharedProps} />);

    for (const slider of screen.getAllByRole("slider", {
      name: "Seek through track",
    })) {
      expect((slider as HTMLInputElement).value).toBe("0");
    }
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
      screen.getByRole("button", { name: "Previous track" }),
    ).toHaveClass(...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "));
    expect(screen.getByRole("button", { name: "Previous track" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Play" })).toHaveClass(
      ...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "),
    );
    expect(screen.getByRole("button", { name: "No next track" })).toHaveClass(
      ...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "),
    );
    expect(screen.getByRole("button", { name: "No next track" })).toHaveAttribute(
      "aria-disabled",
      "true",
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
      screen.getByRole("button", { name: "Previous track" }),
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

  describe("previous and next controls", () => {
    function secondTrack(): TrackType {
      return track({
        id: 43,
        title: "Basalt",
        file_path: "/music/basalt.flac",
        file_name: "basalt.flac",
      });
    }

    it("restarts the only track when previous is pressed", () => {
      const onTrackChange = vi.fn();
      const { audio } = renderAudioPlayer({ onTrackChange });
      const setCurrentTime = observeAudioCurrentTime(audio, 42);

      fireEvent.click(screen.getByRole("button", { name: "Previous track" }));

      expect(onTrackChange).not.toHaveBeenCalled();
      expect(setCurrentTime).toHaveBeenCalledWith(0);
    });

    it("restarts the current track when previous is pressed past the threshold", () => {
      const first = track();
      const current = secondTrack();
      const onTrackChange = vi.fn();
      const { audio } = renderAudioPlayer({
        currentTrack: current,
        tracks: [first, current],
        onTrackChange,
      });
      const setCurrentTime = observeAudioCurrentTime(audio, 10);

      fireEvent.click(screen.getByRole("button", { name: "Previous track" }));

      expect(onTrackChange).not.toHaveBeenCalled();
      expect(setCurrentTime).toHaveBeenCalledWith(0);
    });

    it("navigates to the previous track near the start of playback", () => {
      const first = track();
      const current = secondTrack();
      const onTrackChange = vi.fn();
      const { audio } = renderAudioPlayer({
        currentTrack: current,
        tracks: [first, current],
        onTrackChange,
      });
      const setCurrentTime = observeAudioCurrentTime(audio, 1);

      fireEvent.click(screen.getByRole("button", { name: "Previous track" }));

      expect(onTrackChange).toHaveBeenCalledWith(first);
      expect(setCurrentTime).not.toHaveBeenCalled();
    });

    it("applies the same previous behavior to the media session handler", () => {
      const mediaSession = mockMediaSession();
      const first = track();
      const current = secondTrack();
      const onTrackChange = vi.fn();
      const { audio } = renderAudioPlayer({
        currentTrack: current,
        tracks: [first, current],
        onTrackChange,
      });
      observeAudioCurrentTime(audio, 1);

      const previousHandler = mediaSession.setActionHandler.mock.calls
        .filter(([action, handler]) => action === "previoustrack" && handler)
        .at(-1)?.[1] as (() => void) | undefined;
      if (!previousHandler) {
        throw new Error("previoustrack handler was not registered");
      }

      act(() => {
        previousHandler();
      });

      expect(onTrackChange).toHaveBeenCalledWith(first);
    });

    it("applies the same previous behavior to the keyboard shortcut", () => {
      const onTrackChange = vi.fn();
      const { audio } = renderAudioPlayer({ onTrackChange });
      const setCurrentTime = observeAudioCurrentTime(audio, 42);

      fireEvent.keyDown(window, { key: "p" });

      expect(onTrackChange).not.toHaveBeenCalled();
      expect(setCurrentTime).toHaveBeenCalledWith(0);
    });

    it("keeps the next button focusable but inert on the last track", () => {
      const onTrackChange = vi.fn();
      renderAudioPlayer({ onTrackChange });

      const nextButton = screen.getByRole("button", { name: "No next track" });
      nextButton.focus();

      fireEvent.click(nextButton);

      expect(onTrackChange).not.toHaveBeenCalled();
      expect(nextButton).toHaveAttribute("aria-disabled", "true");
      expect(nextButton).toHaveFocus();
    });
  });

  describe("global keyboard shortcut guards", () => {
    const dialogStub = (
      <div role="dialog" aria-label="Stub dialog">
        <button type="button">Dialog button</button>
      </div>
    );

    it("leaves Space to activate focused controls instead of toggling playback", () => {
      const { audio } = renderAudioPlayer();
      observeAudioCurrentTime(audio, 42);

      const prevButton = screen.getByRole("button", { name: "Previous track" });
      prevButton.focus();

      const playMock = HTMLMediaElement.prototype.play as ReturnType<
        typeof vi.fn
      >;
      const playCallsBefore = playMock.mock.calls.length;

      const defaultNotPrevented = fireEvent.keyDown(prevButton, { key: " " });

      expect(defaultNotPrevented).toBe(true);
      expect(playMock.mock.calls.length).toBe(playCallsBefore);
      expect(HTMLMediaElement.prototype.pause).not.toHaveBeenCalled();
    });

    it("ignores shortcuts fired from inside a foreign dialog", () => {
      const onTrackChange = vi.fn();
      const { audio } = renderAudioPlayer({
        onTrackChange,
        siblings: dialogStub,
      });
      const setCurrentTime = observeAudioCurrentTime(audio, 42);

      const dialogButton = screen.getByRole("button", {
        name: "Dialog button",
      });
      dialogButton.focus();

      fireEvent.keyDown(dialogButton, { key: "p" });
      fireEvent.keyDown(dialogButton, { key: "ArrowRight" });

      expect(onTrackChange).not.toHaveBeenCalled();
      expect(setCurrentTime).not.toHaveBeenCalled();
    });

    it("keeps arrow seeking off buttons outside the player but on inside it", () => {
      const { audio } = renderAudioPlayer({
        siblings: <button type="button">Outside button</button>,
      });
      setAudioNumber(audio, "duration", 120);
      const setCurrentTime = observeAudioCurrentTime(audio, 30);

      const outsideButton = screen.getByRole("button", {
        name: "Outside button",
      });
      outsideButton.focus();
      fireEvent.keyDown(outsideButton, { key: "ArrowRight" });
      expect(setCurrentTime).not.toHaveBeenCalled();

      const prevButton = screen.getByRole("button", { name: "Previous track" });
      prevButton.focus();
      fireEvent.keyDown(prevButton, { key: "ArrowRight" });
      expect(setCurrentTime).toHaveBeenCalledWith(40);
    });

    it("still seeks with arrows from non-interactive targets", () => {
      const { audio } = renderAudioPlayer();
      setAudioNumber(audio, "duration", 120);
      const setCurrentTime = observeAudioCurrentTime(audio, 30);

      fireEvent.keyDown(document.body, { key: "ArrowRight" });

      expect(setCurrentTime).toHaveBeenCalledWith(40);
    });
  });

  describe("live region announcements", () => {
    it("announces track changes while minimized", () => {
      const audioRef = createRef<HTMLAudioElement>();
      const first = track();
      const second = track({ id: 43, title: "Basalt" });
      const sharedProps = {
        tracks: [first, second],
        albumCover: "/covers/7.jpg",
        albumTitle: "Blue Record",
        musicianName: "The Band",
        onTrackChange: vi.fn(),
        audioRef,
        isPlaying: false,
        onPlayStateChange: vi.fn(),
        isExpanded: false,
        onMinimize: vi.fn(),
        onExpand: vi.fn(),
      };

      const { rerender, container } = render(
        <AudioPlayer track={first} {...sharedProps} />,
      );

      rerender(<AudioPlayer track={second} {...sharedProps} />);

      const liveRegion = container.querySelector('[aria-live="polite"]');
      expect(liveRegion).not.toBeNull();
      expect(liveRegion).toHaveTextContent("Now playing: Basalt by The Band");
    });
  });
});
