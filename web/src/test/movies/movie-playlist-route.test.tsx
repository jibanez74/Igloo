import { screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { MOVIES_PER_PAGE } from "@/lib/constants";
import { jsonResponse, requestURL } from "../helpers/api";
import { authUser, nullableInt64, nullableString } from "../helpers/fixtures";
import { renderRoute } from "../helpers/render-route";

function movie(id: number, title: string, year: number) {
  return {
    id,
    title,
    poster_path: nullableString(),
    year: nullableInt64(year),
  };
}

function mockPlaylistFetch(movies: ReturnType<typeof movie>[]) {
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = requestURL(input);

    if (url === "/api/auth/user") {
      return jsonResponse({ error: false, data: { user: authUser() } });
    }

    if (url === "/api/movies/playlists/11") {
      return jsonResponse({
        error: false,
        data: {
          playlist: {
            id: 11,
            user_id: 1,
            name: "Weekend Picks",
            description: nullableString("Two for Saturday."),
            cover_image: nullableString(),
            is_public: false,
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z",
          },
          movie_count: movies.length,
          is_owner: true,
          can_edit: true,
          collaborators: null,
        },
      });
    }

    if (
      url ===
      `/api/movies/playlists/11/movies?page=1&per_page=${MOVIES_PER_PAGE}&sort=asc`
    ) {
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

    return jsonResponse({ error: false, data: {} });
  });

  vi.stubGlobal("fetch", fetchMock);

  return fetchMock;
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

    expect(screen.getByText("Playlist movies")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Sort/ }),
    ).toBeInTheDocument();
    expect(await screen.findByText("Arrival")).toBeInTheDocument();
    expect(screen.getByText("Heat")).toBeInTheDocument();
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
});
