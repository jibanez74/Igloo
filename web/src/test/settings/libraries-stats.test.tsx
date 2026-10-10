import { screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { authUserData } from "../helpers/fixtures";
import { jsonResponse, requestURL } from "../helpers/api";
import { renderRoute } from "../helpers/render-route";

// Only the music counts answer; the shows counts fail, and the movie path is
// blank so its tile never renders. The scan status calls fall through to the
// 500 and show their own unavailable notice.
function mockLibrariesFetch() {
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = requestURL(input);

    if (url === "/api/auth/user") {
      return jsonResponse({
        error: false,
        data: { user: authUserData({ is_admin: true }) },
      });
    }

    if (url === "/api/settings") {
      return jsonResponse({
        error: false,
        data: { movies_dir: "", shows_dir: "/media/shows", music_dir: "/media/music" },
      });
    }

    if (url === "/api/music/stats") {
      return jsonResponse({
        error: false,
        data: { total_albums: 12, total_tracks: 140, total_musicians: 0 },
      });
    }

    return jsonResponse(
      { error: true, message: `Unexpected request: ${url}` },
      500,
    );
  });

  vi.stubGlobal("fetch", fetchMock);
}

describe("library count tiles", () => {
  it("shows an unavailable figure for failed counts and a real zero for an empty one", async () => {
    mockLibrariesFetch();

    await renderRoute("/settings/libraries");

    const shows = await screen.findByLabelText("TV shows library statistics");
    for (const label of ["Shows", "Seasons", "Episodes"]) {
      expect(
        await within(shows).findByText(`${label} count unavailable`),
      ).toBeInTheDocument();
    }
    expect(within(shows).queryByText("0")).not.toBeInTheDocument();

    const music = screen.getByLabelText("Music library statistics");
    expect(await within(music).findByText("12")).toBeInTheDocument();
    expect(within(music).getByText("140")).toBeInTheDocument();
    expect(within(music).getByText("0")).toBeInTheDocument();
    expect(within(music).queryByText(/count unavailable/)).not.toBeInTheDocument();

    expect(screen.queryByLabelText("Movies library statistics")).not.toBeInTheDocument();
  });
});
