import { screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { authUserData } from "../helpers/fixtures";
import { deferredResponse, jsonResponse, requestURL } from "../helpers/api";
import { renderRoute } from "../helpers/render-route";
import { musicScanStatus } from "../helpers/scan-status";

// Only the music counts answer; the shows counts fail, and the movie path is
// blank so its tile never renders. The scan status calls fall through to the
// 500 and show their own unavailable notice.
function mockLibrariesFetch({
  musicScanStart,
}: {
  /** Makes the music library scannable; answers the request that starts it. */
  musicScanStart?: Promise<Response>;
} = {}) {
  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const url = requestURL(input);
    const method = init?.method ?? "GET";

    // A save that never answers: the tests that save are about the time it
    // takes.
    if (url === "/api/settings/libraries" && method === "PUT") {
      return new Promise<Response>(() => {});
    }

    if (url === "/api/settings/scan/music" && musicScanStart) {
      if (method === "POST") return musicScanStart;
      return jsonResponse({
        error: false,
        data: musicScanStatus({ state: "completed" }),
      });
    }

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

describe("clearing a library path", () => {
  // The button leaves with the path it cleared, which used to drop focus on
  // the page body.
  it("moves focus to the path field the button emptied", async () => {
    mockLibrariesFetch();
    const user = userEvent.setup();

    await renderRoute("/settings/libraries");

    const path = await screen.findByLabelText("Music library path");
    screen.getByRole("button", { name: "Clear music library path" }).focus();
    await user.keyboard("{Enter}");

    expect(path).toHaveValue("");
    expect(
      screen.queryByRole("button", { name: "Clear music library path" }),
    ).toBeNull();
    expect(path).toHaveFocus();
  });
});

describe("where focus stays in the library form", () => {
  // Enter in a path field submits the form. Disabled for the length of the
  // save, the field dropped the focus it held.
  it("keeps it in the path field whose Enter saved the form", async () => {
    mockLibrariesFetch();
    const user = userEvent.setup();

    await renderRoute("/settings/libraries");

    const path = await screen.findByLabelText("Music library path");
    await user.type(path, "-moved{Enter}");

    await waitFor(() => expect(path).toHaveAttribute("readonly"));
    expect(path).toBeEnabled();
    expect(path).toHaveFocus();
  });

  // Scan disables itself for as long as the scan takes to start and run.
  it("moves it from Scan to the section's heading, and back if the scan cannot start", async () => {
    const start = deferredResponse();
    mockLibrariesFetch({ musicScanStart: start.promise });
    const user = userEvent.setup();

    await renderRoute("/settings/libraries");

    const scan = await screen.findByRole("button", { name: "Scan music library" });
    await waitFor(() => expect(scan).toBeEnabled());
    scan.focus();
    await user.keyboard("{Enter}");

    await waitFor(() => expect(scan).toBeDisabled());
    expect(screen.getByRole("heading", { name: "Music library" })).toHaveFocus();

    start.resolve(jsonResponse({ error: true, message: "internal_error" }, 500));
    await waitFor(() => expect(scan).toBeEnabled());
    expect(scan).toHaveFocus();
  });
});
