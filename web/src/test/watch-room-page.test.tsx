import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, beforeEach, afterEach, vi } from "vitest";
import { WatchRoomPageContent } from "@/routes/_auth/watch-rooms/$id.lazy";
import type { WatchRoomDetailType } from "@/types";
import { renderWithQueryClient } from "@/test/render";

const navigateMock = vi.fn();
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
        useParams: () => ({ id: "7" }),
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

vi.mock("@/components/VideoPlayer", () => ({
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

vi.mock("@/components/ProgressBar", () => ({
  default: () => <div data-testid="progress-bar" />,
}));

vi.mock("@/components/LiveAnnouncer", () => ({
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

  readyState = FakeWebSocket.CONNECTING;
  sentMessages: string[] = [];
  private listeners = new Map<string, Set<(event: Event | MessageEvent) => void>>();

  constructor(public url: string) {
    FakeWebSocket.instances.push(this);
    this.readyState = FakeWebSocket.OPEN;
    queueMicrotask(() => {
      if (this.readyState === FakeWebSocket.OPEN) {
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
    this.readyState = FakeWebSocket.CLOSED;
    this.dispatch("close", new Event("close"));
  }

  emitMessage(payload: Record<string, unknown>) {
    this.dispatch(
      "message",
      new MessageEvent("message", {
        data: JSON.stringify(payload),
      }),
    );
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
    mockVideoController.reset();
    navigateMock.mockReset();
    useQueryMock.mockReset();
    joinWatchRoomMock.mockReset();
    deleteWatchRoomMock.mockReset();
    showActionFailedMock.mockReset();
    showInfoMock.mockReset();
    showSuccessMock.mockReset();
    globalThis.WebSocket = FakeWebSocket as unknown as typeof WebSocket;
  });

  afterEach(() => {
    globalThis.WebSocket = originalWebSocket;
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
      expect(mockVideoController.currentTime).toBe(37);
      expect(mockVideoController.playCalls).toBe(1);
      expect(
        screen.getByRole("button", { name: /pause playback/i }),
      ).toBeInTheDocument();
    });
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
    expect(screen.getByRole("button", { name: /adjust volume/i })).toBeInTheDocument();
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
