import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import WatchRoomCard from "@/components/watch-room/WatchRoomCard";
import {
  CARD_INTERACTIVE_SURFACE_CLASS,
  CARD_MEDIA_HOVER_CLASS,
  WATCH_ROOM_KEY,
  WATCH_ROOMS_KEY,
} from "@/lib/constants";
import type { ApiResponseType, WatchRoomType } from "@/types";
import { renderWithQueryClient } from "@/test/render";

const deleteWatchRoomMock = vi.fn();
const showActionFailedMock = vi.fn();
const showSuccessMock = vi.fn();

beforeEach(() => {
  deleteWatchRoomMock.mockReset();
  showActionFailedMock.mockReset();
  showSuccessMock.mockReset();
});

vi.mock("@tanstack/react-router", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>(
      "@tanstack/react-router",
    );

  return {
    ...actual,
    Link: ({
      children,
      params,
      to,
      ...props
    }: {
      children: React.ReactNode;
      params?: { id?: string };
      to?: string;
    }) => {
      const href =
        typeof to === "string"
          ? to.replace("$id", params?.id ?? "")
          : "#";

      return (
        <a href={href} {...props}>
          {children}
        </a>
      );
    },
  };
});

vi.mock("@/lib/api", () => ({
  deleteWatchRoom: (...args: unknown[]) => deleteWatchRoomMock(...args),
}));

vi.mock("@/lib/toast-helpers", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/toast-helpers")>(
      "@/lib/toast-helpers",
    );

  return {
    ...actual,
    showActionFailed: (...args: unknown[]) => showActionFailedMock(...args),
    showSuccess: (...args: unknown[]) => showSuccessMock(...args),
  };
});

function buildRoom(overrides: Partial<WatchRoomType> = {}): WatchRoomType {
  return {
    id: 42,
    movie_id: 8,
    movie_title: "Moonfall",
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
    is_owner: true,
    created_at: "2026-04-14T12:00:00Z",
    ...overrides,
  };
}

describe("WatchRoomCard", () => {
  it("uses the shared motion contracts for the card surface and poster", () => {
    const { container } = renderWithQueryClient(
      <WatchRoomCard
        room={buildRoom({
          movie_poster: "/moonfall.jpg",
        })}
      />,
    );

    const card = container.querySelector("article");
    const poster = container.querySelector("img");

    expect(card).toBeTruthy();
    expect(card?.className).toContain(CARD_INTERACTIVE_SURFACE_CLASS);
    expect(poster).toBeTruthy();
    expect(poster?.className).toContain(CARD_MEDIA_HOVER_CLASS);
  });

  it("shows the owner delete affordance only for owners", () => {
    const { rerender } = renderWithQueryClient(
      <WatchRoomCard room={buildRoom({ is_owner: true })} />,
    );

    expect(
      screen.getByRole("button", { name: /close watch room for moonfall/i }),
    ).toBeInTheDocument();

    rerender(<WatchRoomCard room={buildRoom({ is_owner: false })} />);

    expect(
      screen.queryByRole("button", { name: /close watch room for moonfall/i }),
    ).not.toBeInTheDocument();
  });

  it("keeps the confirmation dialog open when delete fails", async () => {
    deleteWatchRoomMock.mockResolvedValue({
      error: true,
      message: "Room deletion failed.",
    });

    const user = userEvent.setup();
    renderWithQueryClient(<WatchRoomCard room={buildRoom()} />);

    await user.click(
      screen.getByRole("button", { name: /close watch room for moonfall/i }),
    );

    expect(screen.getByText("Close watch room?")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Close room" }));

    await waitFor(() => {
      expect(deleteWatchRoomMock).toHaveBeenCalledWith(42);
    });

    expect(showActionFailedMock).toHaveBeenCalledWith(
      "close watch room",
      "Room deletion failed.",
    );
    expect(showSuccessMock).not.toHaveBeenCalled();
    expect(screen.getByText("Close watch room?")).toBeInTheDocument();
  });

  it("removes a deleted room from cached watch-room queries", async () => {
    deleteWatchRoomMock.mockResolvedValue({
      error: false,
      data: {
        deleted: true,
      },
    });

    const user = userEvent.setup();
    const room = buildRoom();
    const otherRoom = buildRoom({
      id: 43,
      movie_id: 9,
      movie_title: "Arrival",
    });
    const { queryClient } = renderWithQueryClient(<WatchRoomCard room={room} />);
    queryClient.setQueryData<ApiResponseType<{ rooms: WatchRoomType[] }>>(
      [WATCH_ROOMS_KEY],
      {
        error: false,
        data: {
          rooms: [room, otherRoom],
        },
      },
    );
    queryClient.setQueryData([WATCH_ROOM_KEY, room.id], {
      error: false,
      data: {
        room,
      },
    });

    await user.click(
      screen.getByRole("button", { name: /close watch room for moonfall/i }),
    );
    await user.click(screen.getByRole("button", { name: "Close room" }));

    await waitFor(() => {
      expect(deleteWatchRoomMock).toHaveBeenCalledWith(42);
    });

    const cachedRooms = queryClient.getQueryData<
      ApiResponseType<{ rooms: WatchRoomType[] }>
    >([WATCH_ROOMS_KEY]);

    expect(cachedRooms?.error).toBe(false);
    if (cachedRooms?.error === false) {
      expect(cachedRooms.data.rooms.map(cachedRoom => cachedRoom.id)).toEqual([
        43,
      ]);
    }
    expect(queryClient.getQueryData([WATCH_ROOM_KEY, room.id])).toBeUndefined();
    expect(showSuccessMock).toHaveBeenCalledWith(
      "Watch room closed",
      "\"Moonfall\" is no longer available.",
    );
    expect(screen.queryByText("Close watch room?")).not.toBeInTheDocument();
  });
});
