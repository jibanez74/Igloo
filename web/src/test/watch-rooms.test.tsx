import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import WatchRooms from "@/components/watch-room/WatchRooms";
import type { WatchRoomType } from "@/types";

const useQueryMock = vi.fn();

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

vi.mock("@/components/shared/LiveAnnouncer", () => ({
  default: ({ message }: { message?: string }) => (
    <div data-testid="live-announcer">{message}</div>
  ),
}));

vi.mock("@/components/watch-room/WatchRoomCard", () => ({
  default: ({ room }: { room: WatchRoomType }) => (
    <article aria-label={`Watch room card for ${room.movie_title}`}>
      {room.movie_title}
    </article>
  ),
}));

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
    ],
    playback_mode: "direct",
    is_owner: true,
    created_at: "2026-04-14T12:00:00Z",
    ...overrides,
  };
}

beforeEach(() => {
  useQueryMock.mockReset();
});

describe("WatchRooms", () => {
  it("shows a loading status while watch rooms are pending", () => {
    useQueryMock.mockReturnValue({
      data: undefined,
      isPending: true,
    });

    render(<WatchRooms />);

    expect(
      screen.getByRole("status", { name: "Loading watch rooms..." }),
    ).toBeInTheDocument();
    expect(screen.getByTestId("live-announcer")).toBeEmptyDOMElement();
  });

  it("renders nothing for a successful empty watch-room list", () => {
    useQueryMock.mockReturnValue({
      data: {
        error: false,
        data: {
          rooms: [],
        },
      },
      isPending: false,
    });

    render(<WatchRooms />);

    expect(
      screen.queryByRole("region", { name: /watch rooms/i }),
    ).not.toBeInTheDocument();
  });

  it("shows server errors without rendering room cards", () => {
    useQueryMock.mockReturnValue({
      data: {
        error: true,
        message: "Watch rooms are unavailable.",
      },
      isPending: false,
    });

    render(<WatchRooms />);

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Watch rooms are unavailable.",
    );
    expect(screen.getByTestId("live-announcer")).toHaveTextContent(
      "Watch rooms are unavailable.",
    );
    expect(
      screen.queryByRole("article", { name: /watch room card/i }),
    ).not.toBeInTheDocument();
  });

  it("announces and renders each available watch room", () => {
    useQueryMock.mockReturnValue({
      data: {
        error: false,
        data: {
          rooms: [
            buildRoom(),
            buildRoom({
              id: 43,
              movie_id: 9,
              movie_title: "Arrival",
              is_owner: false,
            }),
          ],
        },
      },
      isPending: false,
    });

    render(<WatchRooms />);

    expect(
      screen.getByRole("region", { name: "Watch Rooms" }),
    ).toBeInTheDocument();
    expect(screen.getByText("2 rooms")).toBeInTheDocument();
    expect(screen.getByTestId("live-announcer")).toHaveTextContent(
      "2 watch rooms available",
    );
    expect(
      screen.getByRole("article", { name: "Watch room card for Moonfall" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("article", { name: "Watch room card for Arrival" }),
    ).toBeInTheDocument();
  });
});
