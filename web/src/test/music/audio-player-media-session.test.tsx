import { createRef, useRef, useState, type PropsWithChildren, type ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
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
import AudioPlayer from "@/components/playback/AudioPlayer";
import {
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_MEDIA_OVERLAY_ENTER_CLASS,
  MOTION_PLAYER_CHROME_BUTTON_CLASS,
  MOTION_PLAYER_CHROME_ENTER_CLASS,
} from "@/lib/constants";
import type { TrackType } from "@/types";

vi.mock("@/lib/api", async importOriginal => ({
  ...(await importOriginal<typeof import("@/lib/api")>()),
  getLikedTrackIds: vi.fn().mockResolvedValue({
    error: false,
    data: { liked_track_ids: [] },
  }),
}));

// The player subtree queries liked-track ids, so every render needs a
// QueryClientProvider. useState keeps the client stable across rerenders.
function Providers({ children }: PropsWithChildren) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: { retry: false },
          mutations: { retry: false },
        },
      }),
  );

  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

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
    { wrapper: Providers },
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

    const { rerender } = render(<AudioPlayer track={track()} {...sharedProps} />, {
      wrapper: Providers,
    });
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
      { wrapper: Providers },
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
    // A single-track queue has no previous track, so the button restarts.
    expect(
      screen.getByRole("button", { name: "Restart track" }),
    ).toHaveClass(...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "));
    expect(screen.getByRole("button", { name: "Restart track" })).toBeEnabled();
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
      screen.getByRole("button", { name: "Restart track" }),
    ).toHaveClass(...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "));
    expect(screen.getByRole("button", { name: "Play" })).toHaveClass(
      ...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "),
    );
    expect(screen.getByRole("button", { name: "No next track" })).toHaveClass(
      ...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "),
    );
    expect(screen.getByRole("button", { name: "Mute (M)" })).toHaveClass(
      ...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "),
    );
  });

  it("keeps transport controls on the single focus-visible ring recipe (design-system §1.7)", () => {
    renderAudioPlayer({ onClose: vi.fn() });

    for (const name of [
      "Restart track",
      "Play",
      "No next track",
      "Stop playback and close player",
    ]) {
      expect(screen.getByRole("button", { name })).toHaveClass(
        ...FOCUS_VISIBLE_RING_CLASS.split(" "),
      );
    }
  });

  it("keeps play/pause focusable while loading via aria-disabled, and ignores clicks", () => {
    const { audio } = renderAudioPlayer({ onClose: vi.fn() });

    const playButton = screen.getByRole("button", { name: "Play" });
    expect(playButton).not.toHaveAttribute("disabled");
    expect(playButton).not.toHaveAttribute("aria-disabled");

    fireEvent(audio, new Event("loadstart"));

    const loadingButton = screen.getByRole("button", { name: "Loading" });
    expect(loadingButton).not.toHaveAttribute("disabled");
    expect(loadingButton).toHaveAttribute("aria-disabled", "true");
    expect(loadingButton).toHaveAttribute("aria-busy", "true");

    // The click guard replaces `disabled`: no play/pause while loading.
    (HTMLMediaElement.prototype.play as ReturnType<typeof vi.fn>).mockClear();
    fireEvent.click(loadingButton);
    expect(HTMLMediaElement.prototype.play).not.toHaveBeenCalled();
    expect(HTMLMediaElement.prototype.pause).not.toHaveBeenCalled();

    fireEvent(audio, new Event("canplay"));
    expect(
      screen.getByRole("button", { name: "Play" }),
    ).not.toHaveAttribute("aria-disabled");
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
      { wrapper: Providers },
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
    render(<AudioPlayerFocusHarness />, { wrapper: Providers });

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

      fireEvent.click(screen.getByRole("button", { name: "Restart track" }));

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

      // Sync the rendered time so the label reflects the restart behavior.
      act(() => {
        audio.dispatchEvent(new Event("timeupdate"));
      });

      fireEvent.click(screen.getByRole("button", { name: "Restart track" }));

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

    it("toggles playback with Space on controls inside the player chrome", () => {
      const { audio } = renderAudioPlayer();
      observeAudioCurrentTime(audio, 42);

      const prevButton = screen.getByRole("button", { name: "Restart track" });
      prevButton.focus();

      const playMock = HTMLMediaElement.prototype.play as ReturnType<
        typeof vi.fn
      >;
      const playCallsBefore = playMock.mock.calls.length;

      // Video-player parity: Space controls playback anywhere inside the
      // player's own chrome (jsdom audio reports paused, so toggle plays).
      const defaultNotPrevented = fireEvent.keyDown(prevButton, { key: " " });

      expect(defaultNotPrevented).toBe(false);
      expect(playMock.mock.calls.length).toBe(playCallsBefore + 1);
    });

    it("leaves Space to activate focused controls outside the player", () => {
      const { audio } = renderAudioPlayer({
        siblings: <button type="button">Outside button</button>,
      });
      observeAudioCurrentTime(audio, 42);

      const outsideButton = screen.getByRole("button", {
        name: "Outside button",
      });
      outsideButton.focus();

      const playMock = HTMLMediaElement.prototype.play as ReturnType<
        typeof vi.fn
      >;
      const playCallsBefore = playMock.mock.calls.length;

      const defaultNotPrevented = fireEvent.keyDown(outsideButton, {
        key: " ",
      });

      expect(defaultNotPrevented).toBe(true);
      expect(playMock.mock.calls.length).toBe(playCallsBefore);
      expect(HTMLMediaElement.prototype.pause).not.toHaveBeenCalled();
    });

    // Space is claimed by playback inside the chrome, so Enter must still be
    // available to activate the focused control (design-system §3.5).
    it("leaves Enter to activate controls inside the player chrome", () => {
      const onClose = vi.fn();
      renderAudioPlayer({ onClose });

      const closeButton = screen.getByRole("button", {
        name: "Stop playback and close player",
      });
      closeButton.focus();

      const playMock = HTMLMediaElement.prototype.play as ReturnType<
        typeof vi.fn
      >;
      const playCallsBefore = playMock.mock.calls.length;

      const defaultNotPrevented = fireEvent.keyDown(closeButton, {
        key: "Enter",
      });

      // Not consumed by the shortcut layer, so the browser's own button
      // activation still runs.
      expect(defaultNotPrevented).toBe(true);
      expect(playMock.mock.calls.length).toBe(playCallsBefore);
      expect(HTMLMediaElement.prototype.pause).not.toHaveBeenCalled();
    });

    it("unmutes when the volume keys are used", () => {
      const { audio } = renderAudioPlayer();
      audio.muted = true;
      audio.volume = 0.5;

      fireEvent.keyDown(document.body, { key: "ArrowUp" });

      expect(audio.muted).toBe(false);
      expect(audio.volume).toBeGreaterThan(0.5);
    });

    it("mirrors the video player's k/j/l/0 aliases", () => {
      const { audio } = renderAudioPlayer();
      setAudioNumber(audio, "duration", 120);
      const setCurrentTime = observeAudioCurrentTime(audio, 30);
      const playMock = HTMLMediaElement.prototype.play as ReturnType<
        typeof vi.fn
      >;

      // The observed currentTime tracks each write: 30 → 40 → 30 → 0.
      fireEvent.keyDown(document.body, { key: "l" });
      expect(setCurrentTime).toHaveBeenLastCalledWith(40);

      fireEvent.keyDown(document.body, { key: "j" });
      expect(setCurrentTime).toHaveBeenLastCalledWith(30);

      fireEvent.keyDown(document.body, { key: "0" });
      expect(setCurrentTime).toHaveBeenLastCalledWith(0);
      expect(setCurrentTime).toHaveBeenCalledTimes(3);

      const playCallsBefore = playMock.mock.calls.length;
      fireEvent.keyDown(document.body, { key: "k" });
      expect(playMock.mock.calls.length).toBe(playCallsBefore + 1);
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

      const prevButton = screen.getByRole("button", { name: "Restart track" });
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

    it("restarts the track with Home from a non-range target", () => {
      const { audio } = renderAudioPlayer();
      const setCurrentTime = observeAudioCurrentTime(audio, 30);

      const defaultNotPrevented = fireEvent.keyDown(document.body, {
        key: "Home",
      });

      expect(defaultNotPrevented).toBe(false);
      expect(setCurrentTime).toHaveBeenCalledWith(0);
    });

    it("leaves Home on an external tab control unhandled", () => {
      const { audio } = renderAudioPlayer({
        siblings: <button role="tab" type="button">Outside tab</button>,
      });
      const setCurrentTime = observeAudioCurrentTime(audio, 30);
      const outsideTab = screen.getByRole("tab", { name: "Outside tab" });

      outsideTab.focus();
      const defaultNotPrevented = fireEvent.keyDown(outsideTab, {
        key: "Home",
      });

      expect(defaultNotPrevented).toBe(true);
      expect(setCurrentTime).not.toHaveBeenCalled();
    });

    it("leaves Home on the expanded volume slider to the native control", () => {
      const { audio } = renderAudioPlayer({ isExpanded: true });
      const setCurrentTime = observeAudioCurrentTime(audio, 30);
      const volumeSlider = screen.getByRole("slider", { name: "Volume" });
      volumeSlider.focus();

      // jsdom does not perform the range input's native Home action, so the
      // global guard must leave the event unprevented for a browser to do it.
      const defaultNotPrevented = fireEvent.keyDown(volumeSlider, {
        key: "Home",
      });

      expect(defaultNotPrevented).toBe(true);
      expect(setCurrentTime).not.toHaveBeenCalled();
    });

    it("prevents Space without toggling playback while audio is loading", () => {
      const { audio } = renderAudioPlayer();
      const playMock = HTMLMediaElement.prototype.play as ReturnType<
        typeof vi.fn
      >;

      fireEvent(audio, new Event("loadstart"));
      playMock.mockClear();

      const defaultNotPrevented = fireEvent.keyDown(document.body, { key: " " });

      expect(defaultNotPrevented).toBe(false);
      expect(playMock).not.toHaveBeenCalled();
      expect(HTMLMediaElement.prototype.pause).not.toHaveBeenCalled();
    });

    it("keeps letter shortcuts working from the player's own sliders, leaving arrows to the slider", () => {
      const { audio } = renderAudioPlayer();
      setAudioNumber(audio, "duration", 120);
      const setCurrentTime = observeAudioCurrentTime(audio, 30);
      // Sync the component's displayed position/duration with the element.
      fireEvent(audio, new Event("durationchange"));
      fireEvent(audio, new Event("timeupdate"));

      // The mini bar renders the sm+ strip and the mobile strip; either works.
      const [seekSlider] = screen.getAllByRole("slider", {
        name: "Seek through track",
      });
      seekSlider.focus();

      // m must reach the global shortcut even with the slider focused…
      fireEvent.keyDown(seekSlider, { key: "m" });
      expect(audio.muted).toBe(true);

      // …while the global arrow handler stays out of the slider's way
      // (the slider's own handler seeks ±5; the global one would add ±10).
      fireEvent.keyDown(seekSlider, { key: "ArrowRight" });
      expect(setCurrentTime).toHaveBeenCalledTimes(1);
      expect(setCurrentTime).toHaveBeenCalledWith(35);
    });

    it("suppresses shortcuts from text inputs", () => {
      const { audio } = renderAudioPlayer({
        siblings: <input type="text" aria-label="Search stub" />,
      });
      setAudioNumber(audio, "duration", 120);

      const textInput = screen.getByRole("textbox", { name: "Search stub" });
      textInput.focus();
      fireEvent.keyDown(textInput, { key: "m" });

      expect(audio.muted).toBe(false);
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
        { wrapper: Providers },
      );

      rerender(<AudioPlayer track={second} {...sharedProps} />);

      const liveRegion = container.querySelector('[aria-live="polite"]');
      expect(liveRegion).not.toBeNull();
      expect(liveRegion).toHaveTextContent("Now playing: Basalt by The Band");
    });
  });
});
