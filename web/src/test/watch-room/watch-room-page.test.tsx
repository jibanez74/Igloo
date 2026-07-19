import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentType } from "react";
import { describe, expect, it, beforeEach, afterEach, vi } from "vitest";
import { WatchRoomPage as WatchRoomPageContent } from "@/components/watch-room/WatchRoomPage";
import { Route as WatchRoomRoute } from "@/routes/_auth/watch-rooms/$id.lazy";
import {
  MOTION_PLAYER_CHROME_BUTTON_CLASS,
  WATCH_ROOM_SEEK_STEP_SEC,
} from "@/lib/constants";
import type { WatchRoomDetailType } from "@/types";
import { renderWithQueryClient } from "@/test/helpers/render";

const navigateMock = vi.fn();
let routeParamId: number | null = 7;
const useQueryMock = vi.fn();
const joinWatchRoomMock = vi.fn();
const deleteWatchRoomMock = vi.fn();
const showActionFailedMock = vi.fn();
const showInfoMock = vi.fn();
const showSuccessMock = vi.fn();

type MockVideoPlayerProps = {
  videoRef: { current: HTMLVideoElement | null };
  title: string;
  onPlay?: () => void;
  onPause?: () => void;
  onEnded?: () => void;
  onTimeUpdate?: (time: number) => void;
  onDurationChange?: (duration: number) => void;
};

const mockVideoController = {
  element: null as HTMLVideoElement | null,
  readyState: 4,
  duration: 120,
  currentTime: 0,
  paused: true,
  playCalls: 0,
  pauseCalls: 0,
  onPlay: undefined as (() => void) | undefined,
  onPause: undefined as (() => void) | undefined,
  onEnded: undefined as (() => void) | undefined,
  onTimeUpdate: undefined as ((time: number) => void) | undefined,
  onDurationChange: undefined as ((duration: number) => void) | undefined,

  reset() {
    this.element = null;
    this.readyState = 4;
    this.duration = 120;
    this.currentTime = 0;
    this.paused = true;
    this.playCalls = 0;
    this.pauseCalls = 0;
    this.onPlay = undefined;
    this.onPause = undefined;
    this.onEnded = undefined;
    this.onTimeUpdate = undefined;
    this.onDurationChange = undefined;
  },

  attach(node: HTMLVideoElement | null, props: MockVideoPlayerProps) {
    props.videoRef.current = node;

    if (!node) {
      this.element = null;
      return;
    }

    this.element = node;
    this.onPlay = props.onPlay;
    this.onPause = props.onPause;
    this.onEnded = props.onEnded;
    this.onTimeUpdate = props.onTimeUpdate;
    this.onDurationChange = props.onDurationChange;

    Object.defineProperty(node, "readyState", {
      configurable: true,
      get: () => this.readyState,
    });
    Object.defineProperty(node, "duration", {
      configurable: true,
      get: () => this.duration,
    });
    Object.defineProperty(node, "currentTime", {
      configurable: true,
      get: () => this.currentTime,
      set: (value: number) => {
        if (this.readyState < 1) {
          return;
        }
        this.currentTime = value;
        this.onTimeUpdate?.(value);
      },
    });
    Object.defineProperty(node, "paused", {
      configurable: true,
      get: () => this.paused,
    });
    Object.defineProperty(node, "play", {
      configurable: true,
      value: vi.fn(async () => {
        this.playCalls += 1;
        if (this.readyState < 3) {
          throw new Error("media not ready");
        }
        this.paused = false;
        this.onPlay?.();
      }),
    });
    Object.defineProperty(node, "pause", {
      configurable: true,
      value: vi.fn(() => {
        this.pauseCalls += 1;
        this.paused = true;
        this.onPause?.();
      }),
    });
  },

  setReadyState(nextReadyState: number) {
    this.readyState = nextReadyState;
    if (!this.element) {
      return;
    }

    if (nextReadyState >= 1) {
      this.onDurationChange?.(this.duration);
      this.element.dispatchEvent(new Event("loadedmetadata"));
    }
    if (nextReadyState >= 3) {
      this.element.dispatchEvent(new Event("canplay"));
    }
  },
};

vi.mock("@tanstack/react-router", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>(
      "@tanstack/react-router",
    );

  return {
    ...actual,
    createLazyFileRoute:
      () =>
      (options: { component: unknown }) => ({
        ...options,
        useParams: () => ({ id: routeParamId }),
        useNavigate: () => navigateMock,
      }),
    useNavigate: () => navigateMock,
  };
});

vi.mock("@tanstack/react-query", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-query")>(
      "@tanstack/react-query",
    );

  return {
    ...actual,
    useQuery: (...args: unknown[]) => useQueryMock(...args),
  };
});

vi.mock("@/lib/api", () => ({
  deleteWatchRoom: (...args: unknown[]) => deleteWatchRoomMock(...args),
  joinWatchRoom: (...args: unknown[]) => joinWatchRoomMock(...args),
}));

vi.mock("@/lib/toast-helpers", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/toast-helpers")>(
      "@/lib/toast-helpers",
    );

  return {
    ...actual,
    showActionFailed: (...args: unknown[]) => showActionFailedMock(...args),
    showInfo: (...args: unknown[]) => showInfoMock(...args),
    showSuccess: (...args: unknown[]) => showSuccessMock(...args),
  };
});

vi.mock("@/components/playback/VideoPlayer", () => ({
  default: (props: MockVideoPlayerProps) => (
    <video
      data-testid="video-player"
      aria-label={`Video player for ${props.title}`}
      ref={node => {
        mockVideoController.attach(node, props);
      }}
    />
  ),
}));

vi.mock("@/components/playback/ProgressBar", () => ({
  default: () => <div data-testid="progress-bar" />,
}));

vi.mock("@/components/shared/LiveAnnouncer", () => ({
  default: ({ message }: { message?: string }) => (
    <div data-testid="live-announcer">{message}</div>
  ),
}));

class FakeWebSocket {
  static CONNECTING = 0;
  static OPEN = 1;
  static CLOSING = 2;
  static CLOSED = 3;
  static instances: FakeWebSocket[] = [];
  static autoOpen = true;

  readyState = FakeWebSocket.CONNECTING;
  sentMessages: string[] = [];
  url: string;
  private listeners = new Map<string, Set<(event: Event | MessageEvent) => void>>();

  constructor(url: string) {
    this.url = url;
    FakeWebSocket.instances.push(this);
    this.readyState = FakeWebSocket.OPEN;
    queueMicrotask(() => {
      if (FakeWebSocket.autoOpen && this.readyState === FakeWebSocket.OPEN) {
        this.dispatch("open", new Event("open"));
      }
    });
  }

  addEventListener(type: string, listener: (event: Event | MessageEvent) => void) {
    const handlers = this.listeners.get(type) ?? new Set();
    handlers.add(listener);
    this.listeners.set(type, handlers);
  }

  removeEventListener(type: string, listener: (event: Event | MessageEvent) => void) {
    this.listeners.get(type)?.delete(listener);
  }

  send(data: string) {
    this.sentMessages.push(data);
  }

  close() {
    if (this.readyState === FakeWebSocket.CLOSED) {
      return;
    }
    this.readyState = FakeWebSocket.CLOSED;
    this.dispatch("close", new Event("close"));
  }

  emitMessage(payload: Record<string, unknown>) {
    this.emitRawMessage(JSON.stringify(payload));
  }

  emitRawMessage(data: string) {
    this.dispatch(
      "message",
      new MessageEvent("message", {
        data,
      }),
    );
  }

  serverClose() {
    this.close();
  }

  private dispatch(type: string, event: Event | MessageEvent) {
    for (const listener of this.listeners.get(type) ?? []) {
      listener(event);
    }
  }
}

function buildRoom(overrides: Partial<WatchRoomDetailType> = {}): WatchRoomDetailType {
  return {
    id: 7,
    movie_id: 22,
    movie_title: "Arrival",
    movie_poster: null,
    owner: {
      id: 1,
      name: "Room Owner",
      avatar: null,
    },
    members: [
      {
        id: 1,
        name: "Room Owner",
        avatar: null,
      },
      {
        id: 2,
        name: "Invited Guest",
        avatar: null,
      },
    ],
    playback_mode: "direct",
    audio_track: 0,
    subtitle_track: null,
    is_owner: true,
    created_at: "2026-04-14T12:00:00Z",
    ...overrides,
  };
}

function renderRoomPage(room: WatchRoomDetailType) {
  useQueryMock.mockImplementation(() => ({
    data: {
      error: false,
      data: {
        room,
      },
    },
    isPending: false,
    isError: false,
  }));
  joinWatchRoomMock.mockImplementation(async (id: number) => ({
    error: false,
    data: {
      room_id: id,
      joined: true,
    },
  }));

  return renderWithQueryClient(<WatchRoomPageContent roomId={room.id} />);
}

describe("WatchRoomPageContent", () => {
  const originalWebSocket = globalThis.WebSocket;

  beforeEach(() => {
    FakeWebSocket.instances = [];
    FakeWebSocket.autoOpen = true;
    mockVideoController.reset();
    navigateMock.mockReset();
    useQueryMock.mockReset();
    joinWatchRoomMock.mockReset();
    deleteWatchRoomMock.mockReset();
    showActionFailedMock.mockReset();
    showInfoMock.mockReset();
    showSuccessMock.mockReset();
    routeParamId = 7;
    globalThis.WebSocket = FakeWebSocket as unknown as typeof WebSocket;
  });

  afterEach(() => {
    globalThis.WebSocket = originalWebSocket;
    vi.useRealTimers();
  });

  it("does not send a redundant join message when the websocket opens", async () => {
    renderRoomPage(buildRoom({ is_owner: false }));

    await waitFor(() => {
      expect(joinWatchRoomMock).toHaveBeenCalledWith(7);
    });
    await waitFor(() => {
      expect(FakeWebSocket.instances).toHaveLength(1);
    });

    expect(FakeWebSocket.instances[0].sentMessages).toEqual([]);
  });

  it("shows the unavailable state for invalid parsed route ids", () => {
    routeParamId = null;
    const RouteComponent = (
      WatchRoomRoute as unknown as { component: ComponentType }
    ).component;

    renderWithQueryClient(<RouteComponent />);

    expect(screen.getByText("Watch room unavailable")).toBeInTheDocument();
    expect(
      screen.getByText("This watch room link is invalid."),
    ).toBeInTheDocument();
    expect(useQueryMock).not.toHaveBeenCalled();
  });

  it("waits for the media to be ready before applying synced playback", async () => {
    mockVideoController.readyState = 0;
    renderRoomPage(buildRoom({ is_owner: false }));

    await waitFor(() => {
      expect(joinWatchRoomMock).toHaveBeenCalledWith(7);
    });
    await waitFor(() => {
      expect(FakeWebSocket.instances).toHaveLength(1);
    });

    FakeWebSocket.instances[0].emitMessage({
      type: "room_snapshot",
      room_id: 7,
      playback: {
        paused: false,
        position_sec: 37,
        updated_at: "2026-04-18T12:00:00Z",
      },
      connected_user_ids: [1, 2],
    });

    expect(mockVideoController.currentTime).toBe(0);
    expect(mockVideoController.playCalls).toBe(0);
    expect(
      screen.getByRole("button", { name: /play playback/i }),
    ).toBeInTheDocument();

    mockVideoController.setReadyState(4);

    await waitFor(() => {
      expect(mockVideoController.currentTime).toBeGreaterThanOrEqual(37);
      expect(mockVideoController.currentTime).toBeLessThan(38);
      expect(mockVideoController.playCalls).toBe(1);
      expect(
        screen.getByRole("button", { name: /pause playback/i }),
      ).toBeInTheDocument();
    });
  });

  it("ignores playback keyboard shortcuts until the media is playable", async () => {
    mockVideoController.readyState = 0;
    renderRoomPage(buildRoom({ is_owner: false }));

    await waitFor(() => {
      expect(joinWatchRoomMock).toHaveBeenCalledWith(7);
    });

    const event = new KeyboardEvent("keydown", {
      key: "k",
      bubbles: true,
      cancelable: true,
    });
    document.body.dispatchEvent(event);

    expect(event.defaultPrevented).toBe(false);
    expect(mockVideoController.playCalls).toBe(0);
  });

  it("allows playback keyboard shortcuts once the media can play", async () => {
    mockVideoController.readyState = 3;
    renderRoomPage(buildRoom({ is_owner: false }));

    await waitFor(() => {
      expect(joinWatchRoomMock).toHaveBeenCalledWith(7);
    });

    const event = new KeyboardEvent("keydown", {
      key: "k",
      bubbles: true,
      cancelable: true,
    });
    document.body.dispatchEvent(event);

    await waitFor(() => {
      expect(event.defaultPrevented).toBe(true);
      expect(mockVideoController.playCalls).toBe(1);
      expect(
        screen.getByRole("button", { name: /pause playback/i }),
      ).toBeInTheDocument();
    });
  });

  it("updates connected members and announces presence changes", async () => {
    renderRoomPage(buildRoom({ is_owner: false }));

    await waitFor(() => {
      expect(FakeWebSocket.instances).toHaveLength(1);
    });

    const socket = FakeWebSocket.instances[0];
    socket.emitMessage({
      type: "room_snapshot",
      room_id: 7,
      connected_user_ids: [1],
    });

    await waitFor(() => {
      expect(screen.getByText("1 connected now")).toBeInTheDocument();
    });

    const membersPanel = screen.getByText("People in this room").closest("aside");
    if (!membersPanel) {
      throw new Error("members panel was not rendered");
    }
    const ownerRow = within(membersPanel).getByText("Room Owner").closest("li");
    const guestRow = within(membersPanel).getByText("Invited Guest").closest("li");
    if (!ownerRow || !guestRow) {
      throw new Error("member rows were not rendered");
    }

    expect(within(ownerRow).getByText("Connected")).toBeInTheDocument();
    expect(within(guestRow).getByText("Away")).toBeInTheDocument();

    socket.emitMessage({
      type: "member_joined",
      room_id: 7,
      member: {
        id: 2,
        name: "Invited Guest",
        avatar: null,
      },
      connected_user_ids: [1, 2],
    });

    await waitFor(() => {
      expect(screen.getByText("2 connected now")).toBeInTheDocument();
      expect(screen.getByTestId("live-announcer")).toHaveTextContent(
        "Invited Guest joined the room",
      );
    });
    expect(within(guestRow).getByText("Connected")).toBeInTheDocument();

    socket.emitMessage({
      type: "member_left",
      room_id: 7,
      member: {
        id: 2,
        name: "Invited Guest",
        avatar: null,
      },
      connected_user_ids: [1],
    });

    await waitFor(() => {
      expect(screen.getByText("1 connected now")).toBeInTheDocument();
      expect(screen.getByTestId("live-announcer")).toHaveTextContent(
        "Invited Guest left the room",
      );
    });
    expect(within(guestRow).getByText("Away")).toBeInTheDocument();
  });

  it("sends playback events for local play, pause, and seek controls", async () => {
    const user = userEvent.setup();
    renderRoomPage(buildRoom({ is_owner: false }));

    await waitFor(() => {
      expect(FakeWebSocket.instances).toHaveLength(1);
    });

    const socket = FakeWebSocket.instances[0];
    for (const name of [
      `Rewind ${WATCH_ROOM_SEEK_STEP_SEC} seconds`,
      "Play playback",
      `Fast-forward ${WATCH_ROOM_SEEK_STEP_SEC} seconds`,
      "Fullscreen",
    ]) {
      expect(screen.getByRole("button", { name })).toHaveClass(
        ...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "),
      );
    }

    await user.click(screen.getByRole("button", { name: /play playback/i }));

    await waitFor(() => {
      expect(JSON.parse(socket.sentMessages.at(-1) ?? "{}")).toEqual({
        type: "play",
        position_sec: 0,
      });
    });

    await user.click(
      screen.getByRole("button", {
        name: `Fast-forward ${WATCH_ROOM_SEEK_STEP_SEC} seconds`,
      }),
    );

    await waitFor(() => {
      expect(JSON.parse(socket.sentMessages.at(-1) ?? "{}")).toEqual({
        type: "seek",
        position_sec: WATCH_ROOM_SEEK_STEP_SEC,
      });
    });

    await user.click(screen.getByRole("button", { name: /pause playback/i }));

    await waitFor(() => {
      expect(JSON.parse(socket.sentMessages.at(-1) ?? "{}")).toEqual({
        type: "pause",
        position_sec: WATCH_ROOM_SEEK_STEP_SEC,
      });
    });

    await user.click(
      screen.getByRole("button", {
        name: `Rewind ${WATCH_ROOM_SEEK_STEP_SEC} seconds`,
      }),
    );

    await waitFor(() => {
      expect(JSON.parse(socket.sentMessages.at(-1) ?? "{}")).toEqual({
        type: "seek",
        position_sec: 0,
      });
    });
  });

  it("sends heartbeat pings while the realtime socket stays open", async () => {
    const callbacks: { heartbeat?: () => void } = {};
    const setIntervalSpy = vi
      .spyOn(window, "setInterval")
      .mockImplementation((handler: TimerHandler) => {
        if (typeof handler === "function") {
          callbacks.heartbeat = handler as () => void;
        }
        return 1 as unknown as ReturnType<typeof window.setInterval>;
      });
    const clearIntervalSpy = vi
      .spyOn(window, "clearInterval")
      .mockImplementation(() => undefined);

    try {
      renderRoomPage(buildRoom({ is_owner: false }));

      await waitFor(() => {
        expect(FakeWebSocket.instances).toHaveLength(1);
        expect(callbacks.heartbeat).toBeDefined();
      });

      const socket = FakeWebSocket.instances[0];
      const heartbeat = callbacks.heartbeat;
      if (!heartbeat) {
        throw new Error("Expected heartbeat callback to be registered.");
      }
      heartbeat();

      expect(JSON.parse(socket.sentMessages.at(-1) ?? "{}")).toEqual({
        type: "ping",
      });
    } finally {
      setIntervalSpy.mockRestore();
      clearIntervalSpy.mockRestore();
    }
  });

  it("ignores malformed websocket messages and reconnects after an unintentional close", async () => {
    renderRoomPage(buildRoom({ is_owner: false }));

    await waitFor(() => {
      expect(joinWatchRoomMock).toHaveBeenCalledWith(7);
      expect(FakeWebSocket.instances).toHaveLength(1);
    });

    const firstSocket = FakeWebSocket.instances[0];
    firstSocket.emitRawMessage("{not json");
    firstSocket.emitMessage({
      type: "room_snapshot",
      room_id: 7,
      connected_user_ids: [1, 2],
    });

    await waitFor(() => {
      expect(screen.getByText("2 connected now")).toBeInTheDocument();
    });

    const callbacks: { reconnect?: () => void } = {};
    const setTimeoutSpy = vi
      .spyOn(window, "setTimeout")
      .mockImplementation((handler: TimerHandler, timeout?: number) => {
        if (timeout === 1000 && typeof handler === "function") {
          callbacks.reconnect = handler as () => void;
        }
        return 1 as unknown as ReturnType<typeof window.setTimeout>;
      });

    try {
      act(() => {
        firstSocket.serverClose();
      });
      const reconnect = callbacks.reconnect;
      if (!reconnect) {
        throw new Error("Expected reconnect callback to be registered.");
      }
      act(() => {
        reconnect();
      });
    } finally {
      setTimeoutSpy.mockRestore();
    }

    await waitFor(() => {
      expect(joinWatchRoomMock).toHaveBeenCalledTimes(2);
      expect(FakeWebSocket.instances).toHaveLength(2);
    });
    expect(FakeWebSocket.instances[1].url).toContain("/api/watch-rooms/7/ws");
  });

  it("keeps reconnecting with capped backoff after more than five consecutive failures", async () => {
    renderRoomPage(buildRoom({ is_owner: false }));

    await waitFor(() => {
      expect(FakeWebSocket.instances).toHaveLength(1);
    });

    // Subsequent sockets never fire "open", so every close below counts as
    // a consecutive connection failure and the backoff must keep growing.
    FakeWebSocket.autoOpen = false;

    const scheduledDelays: number[] = [];

    for (let failure = 1; failure <= 7; failure++) {
      const socket = FakeWebSocket.instances.at(-1);
      if (!socket) {
        throw new Error("Expected an active socket.");
      }

      const callbacks: { reconnect?: () => void } = {};
      const setTimeoutSpy = vi
        .spyOn(window, "setTimeout")
        .mockImplementation((handler: TimerHandler, timeout?: number) => {
          if (
            typeof handler === "function" &&
            typeof timeout === "number" &&
            timeout >= 1000
          ) {
            callbacks.reconnect = handler as () => void;
            scheduledDelays.push(timeout);
          }
          return 1 as unknown as ReturnType<typeof window.setTimeout>;
        });

      try {
        act(() => {
          socket.serverClose();
        });
        const reconnect = callbacks.reconnect;
        if (!reconnect) {
          throw new Error(
            `Expected a reconnect to be scheduled after failure ${failure}.`,
          );
        }
        act(() => {
          reconnect();
        });
      } finally {
        setTimeoutSpy.mockRestore();
      }

      await waitFor(() => {
        expect(FakeWebSocket.instances).toHaveLength(failure + 1);
      });
    }

    expect(scheduledDelays).toEqual([
      1000, 2000, 4000, 8000, 16_000, 16_000, 16_000,
    ]);
  });

  it("only hard-seeks when remote playback drifts past the sync threshold", async () => {
    renderRoomPage(buildRoom({ is_owner: false }));

    await waitFor(() => {
      expect(FakeWebSocket.instances).toHaveLength(1);
    });

    mockVideoController.currentTime = 10;

    // Drift below the 1.5s threshold: position must be left alone.
    FakeWebSocket.instances[0].emitMessage({
      type: "playback_changed",
      room_id: 7,
      playback: {
        paused: true,
        position_sec: 10.8,
        updated_at: "2026-04-18T12:00:00Z",
      },
    });

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: /play playback/i }),
      ).toBeInTheDocument();
    });
    expect(mockVideoController.currentTime).toBe(10);

    // Drift far past the threshold: the player must hard-seek.
    FakeWebSocket.instances[0].emitMessage({
      type: "playback_changed",
      room_id: 7,
      playback: {
        paused: true,
        position_sec: 40,
        updated_at: "2026-04-18T12:00:05Z",
      },
    });

    await waitFor(() => {
      expect(mockVideoController.currentTime).toBe(40);
    });
  });

  it("compensates for time elapsed between receiving a sync and media readiness", async () => {
    const baseNow = Date.now();
    const nowSpy = vi.spyOn(Date, "now").mockReturnValue(baseNow);

    try {
      mockVideoController.readyState = 0;
      renderRoomPage(buildRoom({ is_owner: false }));

      await waitFor(() => {
        expect(FakeWebSocket.instances).toHaveLength(1);
      });

      FakeWebSocket.instances[0].emitMessage({
        type: "playback_changed",
        room_id: 7,
        playback: {
          paused: false,
          position_sec: 50,
          updated_at: "2026-04-18T12:00:00Z",
        },
      });

      // Media takes two (mocked) seconds to become ready; the applied
      // position must include that elapsed time.
      nowSpy.mockReturnValue(baseNow + 2000);
      mockVideoController.setReadyState(4);

      await waitFor(() => {
        expect(mockVideoController.currentTime).toBe(52);
        expect(mockVideoController.playCalls).toBe(1);
      });
    } finally {
      nowSpy.mockRestore();
    }
  });

  it("surfaces an autoplay-permission error when the browser blocks synced play", async () => {
    renderRoomPage(buildRoom({ is_owner: false }));

    await waitFor(() => {
      expect(FakeWebSocket.instances).toHaveLength(1);
      expect(mockVideoController.element).not.toBeNull();
    });

    const video = mockVideoController.element;
    if (!video) {
      throw new Error("Expected mock video element.");
    }
    Object.defineProperty(video, "play", {
      configurable: true,
      value: vi.fn(async () => {
        throw new DOMException("play blocked", "NotAllowedError");
      }),
    });

    FakeWebSocket.instances[0].emitMessage({
      type: "playback_changed",
      room_id: 7,
      playback: {
        paused: false,
        position_sec: 30,
        updated_at: "2026-04-18T12:00:00Z",
      },
    });

    await waitFor(() => {
      expect(
        screen.getByText(/press play to continue syncing/i),
      ).toBeInTheDocument();
    });
  });

  it("broadcasts a pause to the room when local playback ends", async () => {
    renderRoomPage(buildRoom({ is_owner: false }));

    await waitFor(() => {
      expect(FakeWebSocket.instances).toHaveLength(1);
      expect(mockVideoController.onEnded).toBeDefined();
    });

    mockVideoController.currentTime = 118;
    act(() => {
      mockVideoController.onEnded?.();
    });

    const socket = FakeWebSocket.instances[0];
    await waitFor(() => {
      expect(JSON.parse(socket.sentMessages.at(-1) ?? "{}")).toEqual({
        type: "pause",
        position_sec: 118,
      });
    });
  });

  it("closes the socket intentionally on unmount without scheduling a reconnect", async () => {
    const { unmount } = renderRoomPage(buildRoom({ is_owner: false }));

    await waitFor(() => {
      expect(FakeWebSocket.instances).toHaveLength(1);
    });
    const socket = FakeWebSocket.instances[0];

    const setTimeoutSpy = vi.spyOn(window, "setTimeout");
    try {
      unmount();

      expect(socket.readyState).toBe(FakeWebSocket.CLOSED);
      const reconnectSchedules = setTimeoutSpy.mock.calls.filter(
        ([, timeout]) => typeof timeout === "number" && timeout >= 1000,
      );
      expect(reconnectSchedules).toHaveLength(0);
    } finally {
      setTimeoutSpy.mockRestore();
    }
    expect(FakeWebSocket.instances).toHaveLength(1);
  });

  it("redirects invited members gracefully when the room_deleted event arrives", async () => {
    renderRoomPage(buildRoom({ is_owner: false }));

    await waitFor(() => {
      expect(joinWatchRoomMock).toHaveBeenCalledWith(7);
    });
    await waitFor(() => {
      expect(FakeWebSocket.instances).toHaveLength(1);
    });

    FakeWebSocket.instances[0].emitMessage({
      type: "room_deleted",
      room_id: 7,
    });

    await waitFor(() => {
      expect(showInfoMock).toHaveBeenCalledWith(
        "Watch room closed",
        "\"Arrival\" is no longer available.",
      );
    });
    expect(navigateMock).toHaveBeenCalledWith({ to: "/", replace: true });
  });

  it("shows a controlled error when joining the room throws", async () => {
    useQueryMock.mockImplementation(() => ({
      data: {
        error: false,
        data: {
          room: buildRoom({ is_owner: false }),
        },
      },
      isPending: false,
      isError: false,
    }));
    joinWatchRoomMock.mockRejectedValue(new Error("Network offline"));

    renderWithQueryClient(<WatchRoomPageContent roomId={7} />);

    await waitFor(() => {
      expect(joinWatchRoomMock).toHaveBeenCalledWith(7);
    });

    expect(await screen.findByText("Network offline")).toBeInTheDocument();
    expect(
      screen.getByText(/connecting realtime sync/i),
    ).toBeInTheDocument();
    expect(screen.getByText(/arrival/i)).toBeInTheDocument();
    expect(FakeWebSocket.instances).toHaveLength(0);
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it("renders the real volume control without crashing", async () => {
    renderRoomPage(buildRoom({ is_owner: false }));

    await waitFor(() => {
      expect(joinWatchRoomMock).toHaveBeenCalledWith(7);
    });

    expect(
      screen.getByRole("group", { name: /volume control/i }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /adjust volume/i })).toHaveClass(
      ...MOTION_PLAYER_CHROME_BUTTON_CLASS.split(" "),
    );
    expect(screen.getByText(/arrival/i)).toBeInTheDocument();
  });

  it("redirects only once for owner-initiated deletion", async () => {
    deleteWatchRoomMock.mockResolvedValue({
      error: false,
      data: {
        deleted: true,
      },
    });

    const user = userEvent.setup();
    renderRoomPage(buildRoom({ is_owner: true }));

    await waitFor(() => {
      expect(joinWatchRoomMock).toHaveBeenCalledWith(7);
    });
    await waitFor(() => {
      expect(FakeWebSocket.instances).toHaveLength(1);
    });

    await user.click(
      screen.getByRole("button", { name: /close watch room for arrival/i }),
    );
    await user.click(screen.getByRole("button", { name: "Close room" }));

    await waitFor(() => {
      expect(deleteWatchRoomMock).toHaveBeenCalledWith(7);
    });
    await waitFor(() => {
      expect(showSuccessMock).toHaveBeenCalledWith(
        "Watch room closed",
        "\"Arrival\" is no longer available.",
      );
    });

    expect(navigateMock).toHaveBeenCalledTimes(1);
    expect(navigateMock).toHaveBeenCalledWith({ to: "/", replace: true });

    FakeWebSocket.instances[0].emitMessage({
      type: "room_deleted",
      room_id: 7,
    });

    expect(navigateMock).toHaveBeenCalledTimes(1);
    expect(showInfoMock).not.toHaveBeenCalled();
    expect(showActionFailedMock).not.toHaveBeenCalled();
  });

  it("rejoins and replaces the realtime connection when the mounted page switches rooms", async () => {
    let currentRoom = buildRoom({ id: 7, is_owner: false });

    useQueryMock.mockImplementation(() => ({
      data: {
        error: false,
        data: {
          room: currentRoom,
        },
      },
      isPending: false,
      isError: false,
    }));
    joinWatchRoomMock.mockImplementation(async (id: number) => ({
      error: false,
      data: {
        room_id: id,
        joined: true,
      },
    }));

    const view = renderWithQueryClient(
      <WatchRoomPageContent roomId={currentRoom.id} />,
    );

    await waitFor(() => {
      expect(joinWatchRoomMock).toHaveBeenNthCalledWith(1, 7);
    });
    await waitFor(() => {
      expect(FakeWebSocket.instances).toHaveLength(1);
    });

    const firstSocket = FakeWebSocket.instances[0];
    expect(firstSocket.url).toContain("/api/watch-rooms/7/ws");
    expect(navigateMock).not.toHaveBeenCalled();

    currentRoom = buildRoom({
      id: 8,
      movie_title: "Blade Runner 2049",
      is_owner: false,
    });

    view.rerender(<WatchRoomPageContent roomId={currentRoom.id} />);

    await waitFor(() => {
      expect(joinWatchRoomMock).toHaveBeenNthCalledWith(2, 8);
    });
    await waitFor(() => {
      expect(FakeWebSocket.instances).toHaveLength(2);
    });

    const secondSocket = FakeWebSocket.instances[1];
    expect(firstSocket.readyState).toBe(FakeWebSocket.CLOSED);
    expect(secondSocket.url).toContain("/api/watch-rooms/8/ws");
    expect(secondSocket.readyState).toBe(FakeWebSocket.OPEN);
    expect(joinWatchRoomMock).toHaveBeenCalledTimes(2);
    expect(navigateMock).not.toHaveBeenCalled();
  });

  it("does not rejoin or recreate the socket when the same room gets a new object reference", async () => {
    let currentRoom = buildRoom({ id: 7, is_owner: false });

    useQueryMock.mockImplementation(() => ({
      data: {
        error: false,
        data: {
          room: currentRoom,
        },
      },
      isPending: false,
      isError: false,
    }));
    joinWatchRoomMock.mockImplementation(async (id: number) => ({
      error: false,
      data: {
        room_id: id,
        joined: true,
      },
    }));

    const view = renderWithQueryClient(
      <WatchRoomPageContent roomId={currentRoom.id} />,
    );

    await waitFor(() => {
      expect(joinWatchRoomMock).toHaveBeenCalledWith(7);
    });
    await waitFor(() => {
      expect(FakeWebSocket.instances).toHaveLength(1);
    });

    const firstSocket = FakeWebSocket.instances[0];

    currentRoom = buildRoom({
      id: 7,
      is_owner: false,
      movie_title: "Arrival: Director's Cut",
    });

    view.rerender(<WatchRoomPageContent roomId={currentRoom.id} />);

    await waitFor(() => {
      expect(screen.getByText(/arrival: director's cut/i)).toBeInTheDocument();
    });

    expect(joinWatchRoomMock).toHaveBeenCalledTimes(1);
    expect(FakeWebSocket.instances).toHaveLength(1);
    expect(FakeWebSocket.instances[0]).toBe(firstSocket);
    expect(firstSocket.readyState).toBe(FakeWebSocket.OPEN);
    expect(navigateMock).not.toHaveBeenCalled();
  });
});
