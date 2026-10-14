import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import MovieCard from "@/components/movies/MovieCard";
import AlbumCard from "@/components/music/AlbumCard";
import MusicianCard from "@/components/music/MusicianCard";
import ShowCard from "@/components/shows/ShowCard";
import {
  albumDetailsQueryOpts,
  libraryMovieDetailsQueryOpts,
  musicianDetailsQueryOpts,
  showDetailsQueryOpts,
} from "@/lib/query-opts";
import { deferredResponse, jsonResponse, requestURL } from "../helpers/api";
import { nullableInt64, nullableString } from "../helpers/fixtures";
import { simpleAlbum, simpleMusician } from "../helpers/music";
import { renderWithQueryClient } from "../helpers/render";

vi.mock("@tanstack/react-router", async () =>
  (await import("../helpers/router-link-mock")).routerWithAnchorLinks(),
);
vi.mock(
  "@/hooks/useAudioPlayerActions",
  () => import("../helpers/audio-player"),
);

const movie = (id: number) => ({
  id,
  title: `Movie ${id}`,
  poster_path: nullableString(`/movie-${id}.jpg`),
  year: nullableInt64(2026),
});
const show = (id: number) => ({
  id,
  name: `Show ${id}`,
  poster_path: nullableString(`/show-${id}.jpg`),
  premiere_year: nullableInt64(2026),
});
const album = (id: number) =>
  simpleAlbum({
    id,
    title: `Album ${id}`,
    cover: nullableString(`/api/static/albums/${id}.jpg`),
  });
const musician = (id: number) =>
  simpleMusician({
    id,
    name: `Musician ${id}`,
    thumb: nullableString(`/api/static/musicians/${id}.jpg`),
  });

const cardCases = [
  {
    name: "movie",
    renderCard: (id: number) => <MovieCard movie={movie(id)} />,
    queryKey: (id: number) => libraryMovieDetailsQueryOpts(id).queryKey,
    url: (id: number) => `/api/movies/details/${id}`,
    response: (id: number) => ({ error: false, data: { movie: movie(id) } }),
  },
  {
    name: "show",
    renderCard: (id: number) => <ShowCard show={show(id)} />,
    queryKey: (id: number) => showDetailsQueryOpts(id).queryKey,
    url: (id: number) => `/api/shows/details/${id}`,
    response: (id: number) => ({ error: false, data: { show: show(id) } }),
  },
  {
    name: "album",
    renderCard: (id: number) => <AlbumCard album={album(id)} />,
    queryKey: (id: number) => albumDetailsQueryOpts(id).queryKey,
    url: (id: number) => `/api/music/albums/details/${id}`,
    response: (id: number) => ({
      error: false,
      data: { album: album(id), tracks: [] },
    }),
  },
  {
    name: "musician",
    renderCard: (id: number) => <MusicianCard musician={musician(id)} />,
    queryKey: (id: number) => musicianDetailsQueryOpts(id).queryKey,
    url: (id: number) => `/api/music/musicians/${id}`,
    response: (id: number) => ({
      error: false,
      data: { musician: musician(id), albums: [], tracks: [] },
    }),
  },
];

describe.each(cardCases)(
  "$name card detail prefetch",
  ({ renderCard, queryKey, url, response }) => {
    const id = 104;

    function mockDetailsFetch(ids = [id]) {
      const fetchMock = vi.fn((input: RequestInfo | URL) => {
        const itemId = ids.find((itemId) => requestURL(input) === url(itemId));
        if (itemId !== undefined) return jsonResponse(response(itemId));
        return jsonResponse(
          { error: true, message: `Unexpected request: ${requestURL(input)}` },
          500,
        );
      });
      vi.stubGlobal("fetch", fetchMock);
      return fetchMock;
    }

    it("does not fetch details just by rendering", () => {
      const fetchMock = mockDetailsFetch();
      const { queryClient } = renderWithQueryClient(renderCard(id));

      expect(fetchMock).not.toHaveBeenCalled();
      expect(queryClient.getQueryData(queryKey(id))).toBeUndefined();
    });

    it.each(["card surface", "image", "title"] as const)(
      "prefetches details when hovering the %s",
      async (target) => {
        const user = userEvent.setup();
        const fetchMock = mockDetailsFetch();
        const { queryClient } = renderWithQueryClient(renderCard(id));
        const hoverTargets = {
          "card surface": screen.getByRole("article"),
          image: screen.getByAltText(""),
          title: screen.getByRole("heading"),
        };

        await user.hover(hoverTargets[target]);

        await waitFor(() => {
          expect(queryClient.getQueryData(queryKey(id))).toEqual(response(id));
        });
        expect(fetchMock).toHaveBeenCalledTimes(1);
        expect(requestURL(fetchMock.mock.calls[0][0])).toBe(url(id));
        expect(queryClient.getQueryCache().getAll()).toHaveLength(1);
      },
    );

    it("prefetches details when the detail link receives keyboard focus", async () => {
      const user = userEvent.setup();
      const fetchMock = mockDetailsFetch();
      const { queryClient } = renderWithQueryClient(renderCard(id));

      await user.tab();

      expect(screen.getAllByRole("link")[0]).toHaveFocus();
      await waitFor(() => {
        expect(queryClient.getQueryData(queryKey(id))).toEqual(response(id));
      });
      expect(fetchMock).toHaveBeenCalledTimes(1);
      expect(requestURL(fetchMock.mock.calls[0][0])).toBe(url(id));
      expect(queryClient.getQueryCache().getAll()).toHaveLength(1);
    });

    it("reuses fresh details across repeated hover and focus interactions", async () => {
      const user = userEvent.setup();
      const fetchMock = mockDetailsFetch();
      const { queryClient } = renderWithQueryClient(renderCard(id));
      const card = screen.getByRole("article");

      await user.hover(card);
      await waitFor(() => {
        expect(queryClient.getQueryData(queryKey(id))).toEqual(response(id));
      });
      await user.unhover(card);
      await user.hover(screen.getByRole("heading"));
      await user.tab();
      await user.tab({ shift: true });
      await user.tab();

      expect(fetchMock).toHaveBeenCalledTimes(1);
      expect(queryClient.getQueryState(queryKey(id))?.fetchStatus).toBe("idle");
      expect(queryClient.getQueryData(queryKey(id))).toEqual(response(id));
    });

    it("deduplicates hover and focus while details are still in flight", async () => {
      const user = userEvent.setup();
      const pending = deferredResponse();
      const fetchMock = vi.fn(() => pending.promise);
      vi.stubGlobal("fetch", fetchMock);
      const { queryClient } = renderWithQueryClient(renderCard(id));
      const card = screen.getByRole("article");

      await user.hover(card);
      expect(fetchMock).toHaveBeenCalledTimes(1);
      expect(queryClient.getQueryData(queryKey(id))).toBeUndefined();
      expect(queryClient.getQueryState(queryKey(id))?.fetchStatus).toBe(
        "fetching",
      );

      await user.tab();
      await user.unhover(card);
      await user.hover(screen.getByAltText(""));
      expect(fetchMock).toHaveBeenCalledTimes(1);

      pending.resolve(await jsonResponse(response(id)));
      await waitFor(() => {
        expect(queryClient.getQueryData(queryKey(id))).toEqual(response(id));
        expect(queryClient.getQueryState(queryKey(id))?.fetchStatus).toBe(
          "idle",
        );
      });
      expect(fetchMock).toHaveBeenCalledTimes(1);
    });

    it("keeps different item ids isolated in the cache", async () => {
      const user = userEvent.setup();
      const otherId = id + 1;
      const fetchMock = mockDetailsFetch([id, otherId]);
      const { queryClient } = renderWithQueryClient(
        <>
          {renderCard(id)}
          {renderCard(otherId)}
        </>,
      );
      const cards = screen.getAllByRole("article");

      await user.hover(cards[0]);
      await waitFor(() => {
        expect(queryClient.getQueryData(queryKey(id))).toEqual(response(id));
      });
      expect(queryClient.getQueryData(queryKey(otherId))).toBeUndefined();
      expect(fetchMock).toHaveBeenCalledTimes(1);

      await user.hover(cards[1]);
      await waitFor(() => {
        expect(queryClient.getQueryData(queryKey(otherId))).toEqual(
          response(otherId),
        );
      });
      expect(queryClient.getQueryData(queryKey(id))).toEqual(response(id));
      expect(fetchMock).toHaveBeenCalledTimes(2);
      expect(fetchMock.mock.calls.map(([input]) => requestURL(input))).toEqual([
        url(id),
        url(otherId),
      ]);
      expect(queryClient.getQueryCache().getAll()).toHaveLength(2);
    });
  },
);
