import { expect, test } from "./e2e-test";
import type { Page } from "@playwright/test";
import {
  expectNoHorizontalOverflow,
  expectPageHasNoHorizontalScroll,
  VIEWPORTS,
} from "./e2e-layout";
import type {
  SearchAlbumsResponseType,
  SearchAllResponseType,
  SearchMoviesResponseType,
  SearchMusiciansResponseType,
  SearchShowsResponseType,
  SearchTracksResponseType,
} from "../src/types/search";
import { SEARCH_PER_PAGE } from "../src/lib/constants";
import {
  apiResponse,
  fulfillJSON,
  nullableInt64,
  nullableString,
} from "./e2e-api";
import { mockApi } from "./e2e-mock-api";
import { simpleAlbum, simpleMusician, trackListItem } from "./fixtures/music";
import { seasonEpisodes, showDetails } from "../src/test/helpers/show-details";

const movieResult = {
  id: 7,
  title: "Casino Royale",
  poster_path: nullableString(),
  year: nullableInt64(2006),
  certification: nullableString("PG-13"),
} satisfies SearchAllResponseType["movies"]["results"][number];

const showResult = {
  id: 55,
  name: "Casino Nights",
  poster_path: nullableString(),
  premiere_year: nullableInt64(2001),
  certification: nullableString("TV-MA"),
} satisfies SearchAllResponseType["shows"]["results"][number];

const albumResult = simpleAlbum({
  id: 12,
  title: "Casino Original Soundtrack",
  musician: nullableString("Various Artists"),
  year: nullableInt64(1995),
}) satisfies SearchAllResponseType["albums"]["results"][number];

const musicianResult = simpleMusician({
  id: 22,
  name: "Casino House Band",
  album_count: 2,
  track_count: 18,
}) satisfies SearchAllResponseType["musicians"]["results"][number];

const trackResult = trackListItem({
  id: 33,
  title: "Casino Theme",
  duration: 181,
  album_id: nullableInt64(12),
  album_title: nullableString("Casino Original Soundtrack"),
  musician_id: nullableInt64(22),
  musician_name: nullableString("Casino House Band"),
}) satisfies SearchAllResponseType["tracks"]["results"][number];

const allResults = apiResponse<SearchAllResponseType>({
  query: "Casino",
  movies: {
    results: [movieResult],
    total: 1,
  },
  shows: {
    results: [showResult],
    total: 1,
  },
  albums: {
    results: [albumResult],
    total: 1,
  },
  musicians: {
    results: [musicianResult],
    total: 1,
  },
  tracks: {
    results: [trackResult],
    total: 1,
  },
});

const noResults = apiResponse<SearchAllResponseType>({
  query: "Nowhere",
  movies: { results: [], total: 0 },
  shows: { results: [], total: 0 },
  albums: { results: [], total: 0 },
  musicians: { results: [], total: 0 },
  tracks: { results: [], total: 0 },
});

// The show result's own page, where the top-bar search test starts and ends.
const casinoNights = showDetails();
casinoNights.show = { ...casinoNights.show, id: showResult.id, name: showResult.name };

/** One page of results, as every per-kind search endpoint returns them. */
function searchPage<T>(results: T[]) {
  return {
    query: "Casino",
    results,
    total: results.length,
    page: 1,
    per_page: SEARCH_PER_PAGE,
    total_pages: 1,
  };
}

async function mockSearchApi(page: Page) {
  const requestedSearchRequests: string[] = [];
  const resultsByPath: Record<string, unknown> = {
    "/api/search": allResults,
    "/api/search/movies": apiResponse<SearchMoviesResponseType>(searchPage([movieResult])),
    "/api/search/shows": apiResponse<SearchShowsResponseType>(searchPage([showResult])),
    "/api/search/albums": apiResponse<SearchAlbumsResponseType>(searchPage([albumResult])),
    "/api/search/musicians": apiResponse<SearchMusiciansResponseType>(searchPage([musicianResult])),
    "/api/search/tracks": apiResponse<SearchTracksResponseType>(searchPage([trackResult])),
  };

  await mockApi(page, {
    user: { is_admin: true },
    handle: async ({ route, url, method }) => {
      if (method !== "GET") {
        return false;
      }

      if (url.pathname === "/api/search" && url.searchParams.get("q") !== "Casino") {
        await fulfillJSON(route, noResults);
        return true;
      }

      if (url.pathname === `/api/shows/details/${showResult.id}`) {
        await fulfillJSON(route, apiResponse(casinoNights));
        return true;
      }

      const episodes = url.pathname.match(
        new RegExp(`^/api/shows/${showResult.id}/seasons/(\\d+)/episodes$`),
      );
      if (episodes) {
        await fulfillJSON(route, apiResponse(seasonEpisodes(Number(episodes[1]))));
        return true;
      }

      const results = resultsByPath[url.pathname];
      if (results) {
        await fulfillJSON(route, results);
        return true;
      }

      if (url.pathname === "/api/music/tracks/liked-ids") {
        await fulfillJSON(route, apiResponse({ liked_track_ids: [] }));
        return true;
      }

      return false;
    },
  });

  // Counted once answered: under the dev server's StrictMode every observer
  // mounts twice, and the query aborts the request the first mount started.
  page.on("requestfinished", request => {
    const url = new URL(request.url());
    if (resultsByPath[url.pathname]) {
      requestedSearchRequests.push(`${url.pathname}${url.search}`);
    }
  });

  return {
    requestedSearchRequests,
  };
}

test("search supports keyboard submission, tabs, and responsive layout", async ({
  page,
}) => {
  const { requestedSearchRequests } =
    await mockSearchApi(page);

  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto("/search");

  const searchForm = page.getByRole("search", { name: "Search library" });
  const searchInput = searchForm.getByRole("searchbox", { name: "Search" });

  await expect(searchInput).toBeVisible();
  await searchInput.fill("Casino");
  await searchInput.press("Enter");

  await expect
    .poll(() => new URL(page.url()).searchParams.get("q"))
    .toBe("Casino");
  await expect(page.getByRole("heading", { name: /Search results for/i })).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Casino Royale 2006", exact: true }),
  ).toBeVisible();

  const tablist = page.getByRole("tablist");
  await expect(tablist).toBeVisible();
  await expect(page.getByRole("tab")).toHaveCount(6);
  await expectPageHasNoHorizontalScroll(page);
  await expectNoHorizontalOverflow(
    page.getByRole("main"),
    "search page desktop main",
  );
  await expectNoHorizontalOverflow(tablist, "search tablist desktop");

  await page.getByRole("tab", { name: "Movies" }).click();
  await expect(page).toHaveURL(/tab=movies/);
  await expect(page.getByRole("tabpanel", { name: "Movies" })).toBeVisible();
  await expect(page.getByText("1 movie", { exact: true })).toBeVisible();

  await page.getByRole("tab", { name: "Shows" }).click();
  await expect(page).toHaveURL(/tab=shows/);
  await expect(page.getByRole("tabpanel", { name: "Shows" })).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Casino Nights 2001", exact: true }),
  ).toBeVisible();

  await page.getByRole("tab", { name: "Albums" }).click();
  await expect(page).toHaveURL(/tab=albums/);
  await expect(page.getByRole("tabpanel", { name: "Albums" })).toBeVisible();
  await expect(
    page.getByRole("link", {
      name: "Casino Original Soundtrack by Various Artists",
    }),
  ).toBeVisible();

  await page.getByRole("tab", { name: "Musicians" }).click();
  await expect(page).toHaveURL(/tab=musicians/);
  await expect(page.getByRole("tabpanel", { name: "Musicians" })).toBeVisible();
  await expect(
    page.getByRole("link", {
      name: "Casino House Band, 2 albums, 18 tracks",
    }),
  ).toBeVisible();

  await page.getByRole("tab", { name: "Tracks" }).click();
  await expect(page).toHaveURL(/tab=tracks/);
  await expect(page.getByRole("tabpanel", { name: "Tracks" })).toBeVisible();
  await expect(page.getByRole("list", { name: "Track results" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Play Casino Theme" })).toBeVisible();

  await page.setViewportSize(VIEWPORTS.phone);
  await expectPageHasNoHorizontalScroll(page);
  await expectNoHorizontalOverflow(searchForm, "header search form mobile");
  await expectNoHorizontalOverflow(tablist, "search tablist mobile");
  await expectNoHorizontalOverflow(
    page.getByRole("list", { name: "Track results" }),
    "track results mobile",
  );

  expect(requestedSearchRequests).toEqual([
    "/api/search?q=Casino",
    `/api/search/movies?q=Casino&page=1&per_page=${SEARCH_PER_PAGE}`,
    `/api/search/shows?q=Casino&page=1&per_page=${SEARCH_PER_PAGE}`,
    `/api/search/albums?q=Casino&page=1&per_page=${SEARCH_PER_PAGE}`,
    `/api/search/musicians?q=Casino&page=1&per_page=${SEARCH_PER_PAGE}`,
    `/api/search/tracks?q=Casino&page=1&per_page=${SEARCH_PER_PAGE}`,
  ]);
});

test("the top bar searches from any page, says when nothing matches, and opens a result", async ({
  page,
}) => {
  await mockSearchApi(page);

  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto(`/tv-shows/${showResult.id}`);
  await expect(page.getByRole("heading", { level: 1, name: /Casino Nights/ })).toBeVisible();

  const searchInput = page
    .getByRole("search", { name: "Search library" })
    .getByRole("searchbox", { name: "Search" });

  await searchInput.fill("  Nowhere ");
  await searchInput.press("Enter");

  // The query is trimmed; the All tab and page one are the URL defaults.
  await expect(page).toHaveURL("/search?q=Nowhere");
  await expect(
    page.getByText("No results found for ‘Nowhere’. Try a different search term."),
  ).toBeVisible();
  await expect(searchInput).toHaveValue("Nowhere");

  await searchInput.fill("Casino");
  await searchInput.press("Enter");
  await expect(page).toHaveURL("/search?q=Casino");

  // Going back restores the previous query into the box.
  await page.goBack();
  await expect(page).toHaveURL(/q=Nowhere/);
  await expect(searchInput).toHaveValue("Nowhere");
  await page.goForward();
  await expect(searchInput).toHaveValue("Casino");

  await page.getByRole("link", { name: "Casino Nights 2001", exact: true }).click();
  await expect(page).toHaveURL(`/tv-shows/${showResult.id}`);
  await expect(page.getByRole("heading", { level: 1, name: /Casino Nights/ })).toBeVisible();
  // A page without a query leaves the box empty.
  await expect(searchInput).toHaveValue("");
});
