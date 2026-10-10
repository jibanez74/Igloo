import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { MOVIES_PER_PAGE } from "@/lib/constants";
import { jsonResponse, requestURL } from "../helpers/api";
import { authUser, nullableInt64, nullableString } from "../helpers/fixtures";
import { readDocumentHead, renderRoute } from "../helpers/render-route";

function movie(id: number, title: string, year: number) {
  return {
    id,
    title,
    poster_path: nullableString(),
    year: nullableInt64(year),
  };
}

type MockPlaylist = {
  name: string;
  description?: string;
  movies: ReturnType<typeof movie>[];
  is_owner?: boolean;
  can_edit?: boolean;
};

function mockPlaylistsFetch(playlists: Record<number, MockPlaylist>) {
  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const url = requestURL(input);
    const method = init?.method ?? "GET";

    if (url === "/api/auth/user") {
      return jsonResponse(authUser());
    }

    const detail = url.match(/^\/api\/movies\/playlists\/(\d+)$/);
    const playlist = detail ? playlists[Number(detail[1])] : undefined;
    if (detail && playlist && method === "DELETE") {
      delete playlists[Number(detail[1])];
      return jsonResponse({ error: false, message: "Playlist deleted successfully" });
    }
    if (detail && playlist) {
      return jsonResponse({
        error: false,
        data: {
          playlist: {
            id: Number(detail[1]),
            user_id: 1,
            name: playlist.name,
            description: nullableString(playlist.description),
            cover_image: nullableString(),
            is_public: false,
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z",
          },
          movie_count: playlist.movies.length,
          is_owner: playlist.is_owner ?? true,
          can_edit: playlist.can_edit ?? playlist.is_owner ?? true,
          collaborators: null,
        },
      });
    }

    const removal = url.match(/^\/api\/movies\/playlists\/(\d+)\/movies\/(\d+)$/);
    const trimmed = removal ? playlists[Number(removal[1])] : undefined;
    if (removal && trimmed && method === "DELETE") {
      trimmed.movies = trimmed.movies.filter(
        candidate => candidate.id !== Number(removal[2]),
      );
      return jsonResponse({ error: false, message: "Movie removed from playlist" });
    }

    const list = url.match(
      /^\/api\/movies\/playlists\/(\d+)\/movies\?page=1&per_page=\d+&sort=(asc|desc)$/,
    );
    const listed = list ? playlists[Number(list[1])] : undefined;
    if (list && listed) {
      const movies =
        list[2] === "asc" ? listed.movies : [...listed.movies].reverse();
      return jsonResponse({
        error: false,
        data: {
          movies,
          total: movies.length,
          page: 1,
          per_page: MOVIES_PER_PAGE,
          total_pages: 1,
        },
      });
    }

    if (url === "/api/movies/playlists") {
      return jsonResponse({
        error: false,
        data: { playlists: [] },
      });
    }

    return jsonResponse({ error: false, data: {} });
  });

  vi.stubGlobal("fetch", fetchMock);

  return fetchMock;
}

function mockPlaylistFetch(
  movies: ReturnType<typeof movie>[],
  fields: Omit<MockPlaylist, "name" | "movies"> = {},
) {
  return mockPlaylistsFetch({
    11: {
      name: "Weekend Picks",
      description: "Two for Saturday.",
      movies,
      ...fields,
    },
  });
}

describe("movie playlist route", () => {
  it("composes the header and the shared tab from the playlist", async () => {
    mockPlaylistFetch([movie(1, "Arrival", 2016), movie(2, "Heat", 1995)]);

    await renderRoute("/movies/playlist/11");

    expect(
      await screen.findByRole("heading", { name: "Weekend Picks" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Two for Saturday.")).toBeInTheDocument();
    expect(screen.getByText("2 movies")).toBeInTheDocument();

    const backLink = screen.getByRole("link", { name: "Movie playlists" });
    expect(backLink.getAttribute("href")).toContain("tab=playlists");

    expect(screen.getByRole("button", { name: "Edit playlist" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Delete playlist" })).toBeInTheDocument();

    expect(screen.getByText("Playlist movies")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Sort/ }),
    ).toBeInTheDocument();
    expect(await screen.findByText("Arrival")).toBeInTheDocument();
    expect(screen.getByText("Heat")).toBeInTheDocument();
    await waitFor(() => {
      expect(readDocumentHead()).toMatchObject({
        title: "Weekend Picks - Igloo",
        description: "Movie playlist: Weekend Picks",
      });
    });
  });

  it("hides Edit and Delete from a viewer who does not own the playlist", async () => {
    mockPlaylistFetch([movie(1, "Arrival", 2016)], { is_owner: false });

    await renderRoute("/movies/playlist/11");

    expect(
      await screen.findByRole("heading", { name: "Weekend Picks" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("Arrival")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Edit playlist" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Delete playlist" })).not.toBeInTheDocument();
  });

  it("deletes the playlist after confirming and returns to the playlists tab", async () => {
    const fetchMock = mockPlaylistFetch([movie(1, "Arrival", 2016)]);
    const user = userEvent.setup();

    const { router } = await renderRoute("/movies/playlist/11");

    await user.click(await screen.findByRole("button", { name: "Delete playlist" }));

    const confirm = await screen.findByRole("alertdialog", { name: "Delete playlist" });
    expect(confirm).toHaveTextContent("Are you sure you want to delete “Weekend Picks”?");
    await user.click(within(confirm).getByRole("button", { name: "Delete" }));

    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/movies");
    });
    expect(router.state.location.search).toMatchObject({ tab: "playlists" });
    expect(
      fetchMock.mock.calls.some(
        ([input, init]) =>
          requestURL(input as RequestInfo | URL) === "/api/movies/playlists/11" &&
          (init as RequestInit | undefined)?.method === "DELETE",
      ),
    ).toBe(true);
  });

  it("removes a movie from its card menu and refreshes the count", async () => {
    const fetchMock = mockPlaylistFetch([movie(1, "Arrival", 2016), movie(2, "Heat", 1995)]);
    const user = userEvent.setup();

    await renderRoute("/movies/playlist/11");

    expect(await screen.findByText("Heat")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "More actions for Heat" }));
    await user.click(
      await screen.findByRole("menuitem", { name: "Remove from Playlist" }),
    );

    await waitFor(() => {
      expect(screen.queryByText("Heat")).not.toBeInTheDocument();
    });
    expect(screen.getByText("Arrival")).toBeInTheDocument();
    expect(await screen.findByText("1 movie")).toBeInTheDocument();
    expect(
      fetchMock.mock.calls.some(
        ([input, init]) =>
          requestURL(input as RequestInfo | URL) === "/api/movies/playlists/11/movies/2" &&
          (init as RequestInit | undefined)?.method === "DELETE",
      ),
    ).toBe(true);
  });

  it("gives a viewer who cannot edit the playlist no card menu", async () => {
    mockPlaylistFetch([movie(1, "Arrival", 2016)], { is_owner: false, can_edit: false });

    await renderRoute("/movies/playlist/11");

    expect(await screen.findByText("Arrival")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "More actions for Arrival" }),
    ).not.toBeInTheDocument();
  });

  it("scopes the empty copy to the playlist, not the library", async () => {
    mockPlaylistFetch([]);

    await renderRoute("/movies/playlist/11");

    expect(
      await screen.findByText("No movies in this playlist yet."),
    ).toBeInTheDocument();
    expect(screen.getByText("0 movies")).toBeInTheDocument();
  });

  it("rejects a malformed playlist id without asking the API", async () => {
    const fetchMock = mockPlaylistFetch([]);

    await renderRoute("/movies/playlist/abc");

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("That playlist link is not valid.");
    const backLink = within(alert.parentElement as HTMLElement).getByRole(
      "link",
      { name: "Back to movie playlists" },
    );
    expect(backLink.getAttribute("href")).toContain("tab=playlists");
    expect(
      fetchMock.mock.calls.some(([input]) =>
        requestURL(input as RequestInfo | URL).startsWith("/api/movies/playlists/"),
      ),
    ).toBe(false);
  });

  // A private playlist the viewer was not shared answers 403 "access denied";
  // the reader gets a sentence and a way back, never the server's constant.
  it("words a private playlist as no access, with a way back", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) => {
        const url = requestURL(input);
        if (url === "/api/auth/user") {
          return jsonResponse(authUser());
        }
        if (url === "/api/movies/playlists/11") {
          return jsonResponse({ error: true, message: "access denied" }, 403);
        }
        return jsonResponse({ error: false, data: {} });
      }),
    );

    await renderRoute("/movies/playlist/11");

    expect(
      await screen.findByRole("heading", { level: 1, name: "No access" }),
    ).toBeInTheDocument();
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("You don't have access to this playlist.");
    expect(screen.queryByText(/access denied/)).not.toBeInTheDocument();
    expect(
      within(alert.parentElement as HTMLElement)
        .getByRole("link", { name: "Back to movie playlists" })
        .getAttribute("href"),
    ).toContain("tab=playlists");
  });

  it("starts a fresh page and sort when moving from one playlist to another", async () => {
    const fetchMock = mockPlaylistsFetch({
      11: { name: "Weekend Picks", movies: [movie(1, "Arrival", 2016)] },
      12: { name: "Late Night", movies: [movie(3, "Collateral", 2004)] },
    });
    const user = userEvent.setup();

    const { router } = await renderRoute("/movies/playlist/11");

    expect(
      await screen.findByRole("heading", { name: "Weekend Picks" }),
    ).toBeInTheDocument();
    await user.click(
      screen.getByRole("button", { name: "Sorted A to Z, click to sort Z to A" }),
    );
    expect(
      await screen.findByRole("button", {
        name: "Sorted Z to A, click to sort A to Z",
      }),
    ).toBeInTheDocument();

    await act(async () => {
      await router.navigate({
        to: "/movies/playlist/$id",
        params: { id: "12" },
      });
    });

    expect(
      await screen.findByRole("heading", { name: "Late Night" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Sorted A to Z, click to sort Z to A" }),
    ).toBeInTheDocument();

    const listRequests = fetchMock.mock.calls
      .map(([input]) => requestURL(input as RequestInfo | URL))
      .filter(url => url.startsWith("/api/movies/playlists/12/movies"));
    expect(listRequests).toEqual([
      `/api/movies/playlists/12/movies?page=1&per_page=${MOVIES_PER_PAGE}&sort=asc`,
    ]);
  });
});
