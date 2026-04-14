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
  default: () => <div data-testid="video-player" />,
}));

vi.mock("@/components/ProgressBar", () => ({
  default: () => <div data-testid="progress-bar" />,
}));

vi.mock("@/components/VolumeControl", () => ({
  default: () => <div data-testid="volume-control" />,
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
  useQueryMock.mockReturnValue({
    data: {
      error: false,
      data: {
        room,
      },
    },
    isPending: false,
    isError: false,
  });
  joinWatchRoomMock.mockResolvedValue({
    error: false,
    data: {
      room_id: room.id,
      joined: true,
    },
  });

  return renderWithQueryClient(<WatchRoomPageContent roomId={room.id} />);
}

describe("WatchRoomPageContent", () => {
  const originalWebSocket = globalThis.WebSocket;

  beforeEach(() => {
    FakeWebSocket.instances = [];
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
});
