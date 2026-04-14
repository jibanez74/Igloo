import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import WatchRoomCard from "@/components/WatchRoomCard";
import type { WatchRoomType } from "@/types";
import { renderWithQueryClient } from "@/test/render";

const deleteWatchRoomMock = vi.fn();
const showActionFailedMock = vi.fn();
const showSuccessMock = vi.fn();

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
});
