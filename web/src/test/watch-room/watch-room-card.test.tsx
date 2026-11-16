import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import WatchRoomCard from "@/components/watch-room/WatchRoomCard";
import {
  CARD_SURFACE_CLASS,
  CARD_MEDIA_HOVER_CLASS,
  WATCH_ROOM_KEY,
  WATCH_ROOMS_KEY,
} from "@/lib/constants";
import type { ApiResponseType, WatchRoomType } from "@/types";
import { renderWithQueryClient } from "@/test/helpers/render";

const deleteWatchRoomMock = vi.fn();
const leaveWatchRoomMock = vi.fn();
const updateWatchRoomMembersMock = vi.fn();
const getWatchRoomInviteUsersMock = vi.fn();
const showActionFailedMock = vi.fn();
const showSuccessMock = vi.fn();
const showUpdatedMock = vi.fn();

beforeEach(() => {
  deleteWatchRoomMock.mockReset();
  leaveWatchRoomMock.mockReset();
  updateWatchRoomMembersMock.mockReset();
  getWatchRoomInviteUsersMock.mockReset();
  getWatchRoomInviteUsersMock.mockResolvedValue({
    error: false,
    data: {
      users: [
        { id: 2, name: "Invited Guest", email: "guest@example.test", avatar: null },
        { id: 3, name: "Third Viewer", email: "third@example.test", avatar: null },
      ],
    },
  });
  showActionFailedMock.mockReset();
  showSuccessMock.mockReset();
  showUpdatedMock.mockReset();
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

// Radix only mounts the avatar <img> once the browser has loaded it, which
// jsdom never does; render it outright so its src can be read.
vi.mock("@/components/ui/avatar", () => ({
  Avatar: ({ children }: { children: React.ReactNode }) => <span>{children}</span>,
  AvatarImage: (props: { src?: string; alt?: string }) => <img {...props} />,
  AvatarFallback: ({ children }: { children: React.ReactNode }) => (
    <span>{children}</span>
  ),
}));

vi.mock("@/lib/api", async () => ({
  ...(await vi.importActual<typeof import("@/lib/api")>("@/lib/api")),
  deleteWatchRoom: (...args: unknown[]) => deleteWatchRoomMock(...args),
  leaveWatchRoom: (...args: unknown[]) => leaveWatchRoomMock(...args),
  updateWatchRoomMembers: (...args: unknown[]) =>
    updateWatchRoomMembersMock(...args),
  getWatchRoomInviteUsers: (...args: unknown[]) =>
    getWatchRoomInviteUsersMock(...args),
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
    showUpdated: (...args: unknown[]) => showUpdatedMock(...args),
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
    // The media-card surface every card wears (design-system §3.2).
    expect(card).toHaveClass(...CARD_SURFACE_CLASS.split(" "));
    expect(
      screen.getByRole("link", { name: /^Join watch room for/ }),
    ).toHaveAttribute("href", expect.stringContaining("/watch-rooms/"));
    expect(poster).toBeTruthy();
    expect(poster?.className).toContain(CARD_MEDIA_HOVER_CLASS);
    // The 2:3 box must not stretch to the card height, which cropped it.
    expect(poster?.parentElement).toHaveClass("aspect-2/3", "self-start");
  });

  it("shows member avatars from the stored URL, uploaded or external", () => {
    // The server stores the full URL, so the card must not prefix it again.
    renderWithQueryClient(
      <WatchRoomCard
        room={buildRoom({
          members: [
            { id: 1, name: "Room Owner", avatar: "/api/static/avatars/1.webp" },
            {
              id: 2,
              name: "Invited Guest",
              avatar: "https://example.com/guest.png",
            },
          ],
        })}
      />,
    );

    expect(screen.getByRole("img", { name: "Room Owner" })).toHaveAttribute(
      "src",
      "/api/static/avatars/1.webp",
    );
    expect(screen.getByRole("img", { name: "Invited Guest" })).toHaveAttribute(
      "src",
      "https://example.com/guest.png",
    );
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
      expect.objectContaining({ error: true, message: "Room deletion failed." }),
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

  it("offers an invitee Leave instead of the owner's edit and close", () => {
    renderWithQueryClient(<WatchRoomCard room={buildRoom({ is_owner: false })} />);

    expect(
      screen.getByRole("button", { name: "Leave watch room for Moonfall" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /edit invitees for/i }),
    ).not.toBeInTheDocument();
  });

  it("drops a room the invitee left from the cached rooms", async () => {
    leaveWatchRoomMock.mockResolvedValue({ error: false, data: { left: true } });
    const user = userEvent.setup();
    const room = buildRoom({ is_owner: false });
    const { queryClient } = renderWithQueryClient(<WatchRoomCard room={room} />);
    queryClient.setQueryData([WATCH_ROOMS_KEY], {
      error: false,
      data: { rooms: [room, buildRoom({ id: 43 })] },
    });

    await user.click(
      screen.getByRole("button", { name: "Leave watch room for Moonfall" }),
    );
    expect(screen.getByText("Leave watch room?")).toBeInTheDocument();
    expect(screen.getByText(/only room owner can invite you back/i)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Leave room" }));

    await waitFor(() => {
      expect(leaveWatchRoomMock).toHaveBeenCalledWith(42);
    });
    await waitFor(() => {
      const cached = queryClient.getQueryData<
        ApiResponseType<{ rooms: WatchRoomType[] }>
      >([WATCH_ROOMS_KEY]);
      expect(
        cached?.error === false ? cached.data.rooms.map(item => item.id) : null,
      ).toEqual([43]);
    });
    expect(showSuccessMock).toHaveBeenCalledWith(
      "Left watch room",
      "\"Moonfall\" is off your list.",
    );
  });

  it("keeps the leave dialog open when leaving fails", async () => {
    leaveWatchRoomMock.mockResolvedValue({ error: true, message: "access denied" });
    const user = userEvent.setup();
    renderWithQueryClient(<WatchRoomCard room={buildRoom({ is_owner: false })} />);

    await user.click(
      screen.getByRole("button", { name: "Leave watch room for Moonfall" }),
    );
    await user.click(screen.getByRole("button", { name: "Leave room" }));

    await waitFor(() => {
      expect(showActionFailedMock).toHaveBeenCalledWith(
        "leave the watch room",
        expect.objectContaining({ message: "access denied" }),
      );
    });
    expect(screen.getByText("Leave watch room?")).toBeInTheDocument();
  });

  // The owner swaps the current guest for another user: the dialog opens on
  // the room's invitees, saves only a change, and writes the result back.
  it("lets the owner change who is invited", async () => {
    const updatedMembers = [
      { id: 1, name: "Room Owner", avatar: null },
      { id: 3, name: "Third Viewer", avatar: null },
    ];
    updateWatchRoomMembersMock.mockResolvedValue({
      error: false,
      data: { room: { ...buildRoom(), members: updatedMembers } },
    });
    const user = userEvent.setup();
    const room = buildRoom();
    const { queryClient } = renderWithQueryClient(<WatchRoomCard room={room} />);
    queryClient.setQueryData([WATCH_ROOMS_KEY], {
      error: false,
      data: { rooms: [room] },
    });

    await user.click(screen.getByRole("button", { name: "Edit invitees for Moonfall" }));
    const guest = await screen.findByRole("checkbox", { name: "Invite Invited Guest" });
    expect(guest).toBeChecked();
    const save = screen.getByRole("button", { name: "Save invitees" });
    expect(save).toBeDisabled();

    await user.click(guest);
    await user.click(screen.getByRole("checkbox", { name: "Invite Third Viewer" }));
    expect(save).toBeEnabled();
    await user.click(save);

    await waitFor(() => {
      expect(updateWatchRoomMembersMock).toHaveBeenCalledWith(42, {
        invited_user_ids: [3],
      });
    });
    await waitFor(() => {
      expect(screen.queryByText("Edit invitees")).not.toBeInTheDocument();
    });
    const cached = queryClient.getQueryData<
      ApiResponseType<{ rooms: WatchRoomType[] }>
    >([WATCH_ROOMS_KEY]);
    expect(cached?.error === false ? cached.data.rooms[0].members : null).toEqual(
      updatedMembers,
    );
    expect(showUpdatedMock).toHaveBeenCalledWith(
      "Invitees",
      "The room for \"Moonfall\" is up to date.",
    );
  });

  it("keeps the edit dialog open when saving fails", async () => {
    updateWatchRoomMembersMock.mockResolvedValue({
      error: true,
      message: "one or more invited users do not exist",
    });
    const user = userEvent.setup();
    renderWithQueryClient(<WatchRoomCard room={buildRoom()} />);

    await user.click(screen.getByRole("button", { name: "Edit invitees for Moonfall" }));
    await user.click(await screen.findByRole("checkbox", { name: "Invite Third Viewer" }));
    await user.click(screen.getByRole("button", { name: "Save invitees" }));

    await waitFor(() => {
      expect(showActionFailedMock).toHaveBeenCalledWith(
        "update the invitees",
        expect.objectContaining({ message: "one or more invited users do not exist" }),
      );
    });
    expect(screen.getByText("Edit invitees")).toBeInTheDocument();
  });
});
