import { act, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { MOVIES_PER_PAGE } from "@/lib/constants";
import { deferredResponse, jsonResponse, requestURL, stubApi, type StubApiRoute } from "../helpers/api";
import type { UserSummaryType } from "@/types";
import { nullableInt64, nullableString, userSummary } from "../helpers/fixtures";
import { moviePlaylist, moviePlaylistDetail } from "../helpers/movies";
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
  owner?: UserSummaryType;
};

function mockPlaylistsFetch(playlists: Record<number, MockPlaylist>) {
  const routes: Record<string, StubApiRoute> = {
    "/api/movies/playlists": { playlists: [] },
    // Deleting a playlist returns to the movies page, whose header counts the
    // library.
    "/api/movies/stats": { total_movies: 0 },
  };

  for (const [key, initial] of Object.entries(playlists)) {
    const id = Number(key);
    const base = `/api/movies/playlists/${id}`;

    routes[`GET ${base}`] = () => {
      const playlist = playlists[id];
      if (!playlist) {
        return jsonResponse({ error: true, message: "playlist not found" }, 404);
      }
      return moviePlaylistDetail({
        playlist: moviePlaylist({
          id,
          name: playlist.name,
          description: nullableString(playlist.description),
        }),
        movie_count: playlist.movies.length,
        is_owner: playlist.is_owner ?? true,
        can_edit: playlist.can_edit ?? playlist.is_owner ?? true,
        owner: playlist.owner ?? userSummary(),
      });
    };
    routes[`DELETE ${base}`] = () => {
      delete playlists[id];
      return jsonResponse({ error: false, message: "Playlist deleted successfully" });
    };
    routes[`GET ${base}/movies`] = ({ search }) => {
      const movies = playlists[id]?.movies ?? [];
      const page = Number(search.get("page"));
      const perPage = Number(search.get("per_page"));
      const sort = search.get("sort");
      // The page always asks for a page, a size and a direction; a request
      // missing any of them is a bug the test should see, not an empty page.
      if (!(page >= 1) || !(perPage >= 1) || (sort !== "asc" && sort !== "desc")) {
        return jsonResponse({ error: true, message: `bad page query: ${search}` }, 400);
      }
      const sorted = sort === "asc" ? movies : [...movies].reverse();
      return {
        movies: sorted.slice((page - 1) * perPage, page * perPage),
        total: sorted.length,
        page,
        per_page: perPage,
        total_pages: Math.max(1, Math.ceil(sorted.length / perPage)),
      };
    };
    for (const listed of initial.movies) {
      // Pointing at a card prefetches its details; nothing here reads them.
      routes[`GET /api/movies/details/${listed.id}`] = {
        movie: listed,
        cast: [],
        crew: [],
        genres: [],
        production_companies: [],
        extra_videos: [],
      };
      routes[`DELETE ${base}/movies/${listed.id}`] = () => {
        initial.movies = initial.movies.filter(
          candidate => candidate.id !== listed.id,
        );
        return jsonResponse({ error: false, message: "Movie removed from playlist" });
      };
    }
  }

  return stubApi(routes);
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
    expect(screen.queryByText(/^By /)).not.toBeInTheDocument();

    const backLink = screen.getByRole("link", { name: "Movie playlists" });
    expect(backLink.getAttribute("href")).toContain("tab=playlists");

    // 32 px ghost buttons: the bare text links they replace were 16 px tall
    // on a phone, under the 24 px target minimum.
    const edit = screen.getByRole("button", { name: "Edit playlist" });
    expect(edit).toHaveAttribute("data-variant", "ghost-muted");
    expect(edit).toHaveAttribute("data-size", "sm");
    const remove = screen.getByRole("button", { name: "Delete playlist" });
    expect(remove).toHaveAttribute("data-variant", "ghost-destructive");
    expect(remove).toHaveAttribute("data-size", "sm");

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

  it("names the owner instead of Edit and Delete for a viewer who does not own the playlist", async () => {
    mockPlaylistFetch([movie(1, "Arrival", 2016)], {
      is_owner: false,
      owner: userSummary({ id: 2, name: "Riley" }),
    });

    await renderRoute("/movies/playlist/11");

    expect(
      await screen.findByRole("heading", { name: "Weekend Picks" }),
    ).toBeInTheDocument();
    expect(await screen.findByText("Arrival")).toBeInTheDocument();
    expect(screen.getByText("By Riley")).toBeInTheDocument();
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
    // The menu's trigger left with the card, so focus moves to the heading
    // rather than falling to the document.
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Weekend Picks" })).toHaveFocus();
    });
    expect(
      fetchMock.mock.calls.some(
        ([input, init]) =>
          requestURL(input as RequestInfo | URL) === "/api/movies/playlists/11/movies/2" &&
          (init as RequestInit | undefined)?.method === "DELETE",
      ),
    ).toBe(true);
  });

  it("keeps focus on the heading while removal is pending and after it fails", async () => {
    const removalRequest = deferredResponse();
    const fetchMock = mockPlaylistFetch([movie(1, "Arrival", 2016), movie(2, "Heat", 1995)]);
    vi.stubGlobal("fetch", vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      if (
        requestURL(input) === "/api/movies/playlists/11/movies/2" &&
        init?.method === "DELETE"
      ) {
        return removalRequest.promise;
      }
      return fetchMock(input, init);
    }));
    const user = userEvent.setup();

    await renderRoute("/movies/playlist/11");

    const trigger = await screen.findByRole("button", { name: "More actions for Heat" });
    const heading = screen.getByRole("heading", { name: "Weekend Picks" });
    await user.click(trigger);
    await user.click(
      await screen.findByRole("menuitem", { name: "Remove from Playlist" }),
    );

    await waitFor(() => {
      expect(trigger).toBeDisabled();
      expect(screen.queryByRole("menu")).not.toBeInTheDocument();
      expect(heading).toHaveFocus();
    });
    expect(screen.getByText("Heat")).toBeInTheDocument();

    removalRequest.resolve(jsonResponse({ error: true, message: "Removal failed" }, 500));

    await waitFor(() => {
      expect(trigger).toBeEnabled();
    });
    expect(screen.getByText("Heat")).toBeInTheDocument();
    expect(screen.getByText("2 movies")).toBeInTheDocument();
    expect(heading).toHaveFocus();
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
    // Nothing to order, but the row stays for its "Playlist movies" slot.
    expect(screen.getByText("Playlist movies")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Sorted/ })).not.toBeInTheDocument();
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
    stubApi({
      "/api/movies/playlists/11": () =>
        jsonResponse({ error: true, message: "access denied" }, 403),
      // The movie list loads beside the playlist and is refused the same way.
      "/api/movies/playlists/11/movies": () =>
        jsonResponse({ error: true, message: "access denied" }, 403),
    });

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
      11: {
        name: "Weekend Picks",
        movies: [movie(1, "Arrival", 2016), movie(2, "Heat", 1995)],
      },
      12: {
        name: "Late Night",
        movies: [movie(3, "Collateral", 2004), movie(4, "Thief", 1981)],
      },
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
  describe("page and sort in the URL", () => {
    // Two pages: MOVIES_PER_PAGE on the first, six on the second.
    const manyMovies = Array.from({ length: MOVIES_PER_PAGE + 6 }, (_, index) =>
      movie(index + 1, `Movie ${String(index + 1).padStart(2, "0")}`, 2000),
    );

    const listRequests = (fetchMock: ReturnType<typeof mockPlaylistFetch>) =>
      fetchMock.mock.calls
        .map(([input]) => requestURL(input as RequestInfo | URL))
        .filter(url => url.startsWith("/api/movies/playlists/11/movies"));

    it("opens on the page and sort the link names", async () => {
      const fetchMock = mockPlaylistFetch(manyMovies);

      await renderRoute("/movies/playlist/11?page=2&sort=desc");

      // Descending, the second page holds the six lowest titles.
      expect(await screen.findByText("Movie 06")).toBeInTheDocument();
      expect(screen.queryByText("Movie 07")).not.toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Sorted Z to A, click to sort A to Z" }),
      ).toBeInTheDocument();
      expect(listRequests(fetchMock)).toEqual([
        `/api/movies/playlists/11/movies?page=2&per_page=${MOVIES_PER_PAGE}&sort=desc`,
      ]);
    });

    it("writes a page change and a sort change to the URL", async () => {
      mockPlaylistFetch(manyMovies);
      const user = userEvent.setup();

      const { router } = await renderRoute("/movies/playlist/11");

      expect(router.state.location.search).toEqual({});
      await user.click(await screen.findByRole("button", { name: "Go to page 2" }));
      await waitFor(() => {
        expect(router.state.location.search).toEqual({ page: 2 });
      });
      expect(await screen.findByText("Movie 30")).toBeInTheDocument();

      // Re-sorting starts over on the first page, which the URL leaves out.
      await user.click(
        screen.getByRole("button", { name: "Sorted A to Z, click to sort Z to A" }),
      );
      await waitFor(() => {
        expect(router.state.location.search).toEqual({ sort: "desc" });
      });
      expect(await screen.findByText("Movie 07")).toBeInTheDocument();
      expect(screen.queryByText("Movie 06")).not.toBeInTheDocument();
    });

    it("walks an out-of-range page back to the last real page", async () => {
      mockPlaylistFetch(manyMovies);

      const { router } = await renderRoute("/movies/playlist/11?page=9");

      await waitFor(() => {
        expect(router.state.location.search).toEqual({ page: 2 });
      });
      expect(await screen.findByText("Movie 30")).toBeInTheDocument();
    });
  });
});
