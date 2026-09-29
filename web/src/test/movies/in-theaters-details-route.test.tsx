import { screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { jsonResponse, requestURL } from "../helpers/api";
import { authUser } from "../helpers/fixtures";
import { readDocumentHead, renderRoute } from "../helpers/render-route";

function mockInTheatersFetch(movie: unknown) {
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = requestURL(input);

    if (url === "/api/auth/user") {
      return jsonResponse(authUser());
    }

    if (url.startsWith("/api/tmdb/movies/")) {
      return jsonResponse({ error: false, data: { movie } });
    }

    return jsonResponse({ error: false, data: {} });
  });

  vi.stubGlobal("fetch", fetchMock);

  return fetchMock;
}

describe("in-theaters details head", () => {
  it("titles the page from the TMDB movie the loader fetched", async () => {
    mockInTheatersFetch({
      id: 550,
      title: "Fight Club",
      release_date: "1999-10-15",
      overview: "An insomniac office worker forms an underground club.",
    });

    await renderRoute("/movies/in-theaters/550");

    await waitFor(() => {
      expect(readDocumentHead()).toMatchObject({
        title: "Fight Club (1999) - Igloo",
        description: "An insomniac office worker forms an underground club.",
        descriptionCount: 1,
      });
    });
  });

  it("falls back to a generic title when TMDB answers without a movie", async () => {
    mockInTheatersFetch(null);

    await renderRoute("/movies/in-theaters/550");

    await screen.findByRole("alert");
    expect(readDocumentHead().title).toBe("Movie - Igloo");
  });
});

describe("in-theaters details guards", () => {
  // This branch used to render a bare heading with no link out.
  it("offers a way back when TMDB answers without a movie", async () => {
    mockInTheatersFetch(null);

    await renderRoute("/movies/in-theaters/550");

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Movie not found.");
    expect(screen.getByRole("link", { name: "Back to Home" })).toHaveAttribute(
      "href",
      "/",
    );
  });

  it("rejects a malformed movie id without asking TMDB", async () => {
    const fetchMock = mockInTheatersFetch(null);

    await renderRoute("/movies/in-theaters/abc");

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("That movie link is not valid.");
    expect(
      fetchMock.mock.calls.some(([input]) =>
        requestURL(input as RequestInfo | URL).startsWith("/api/tmdb/movies/"),
      ),
    ).toBe(false);
  });
});
