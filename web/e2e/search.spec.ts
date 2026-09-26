import { expect, test, type Page } from "@playwright/test";
import {
  assertMockSuiteClean,
  trackBrowserIssues,
} from "./e2e-browser-issues";
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

const albumResult = {
  id: 12,
  title: "Casino Original Soundtrack",
  cover: nullableString(),
  musician: nullableString("Various Artists"),
  year: nullableInt64(1995),
} satisfies SearchAllResponseType["albums"]["results"][number];

const musicianResult = {
  id: 22,
  name: "Casino House Band",
  thumb: nullableString(),
  album_count: 2,
  track_count: 18,
} satisfies SearchAllResponseType["musicians"]["results"][number];

const trackResult = {
  id: 33,
  title: "Casino Theme",
  duration: 181,
  codec: "flac",
  bit_rate: 900000,
  album_id: nullableInt64(12),
  album_title: nullableString("Casino Original Soundtrack"),
  album_cover: nullableString(),
  musician_id: nullableInt64(22),
  musician_name: nullableString("Casino House Band"),
} satisfies SearchAllResponseType["tracks"]["results"][number];

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

const movieResults = apiResponse<SearchMoviesResponseType>({
  query: "Casino",
  results: [movieResult],
  total: 1,
  page: 1,
  per_page: SEARCH_PER_PAGE,
  total_pages: 1,
});

const showResults = apiResponse<SearchShowsResponseType>({
  query: "Casino",
  results: [showResult],
  total: 1,
  page: 1,
  per_page: SEARCH_PER_PAGE,
  total_pages: 1,
});

const albumResults = apiResponse<SearchAlbumsResponseType>({
  query: "Casino",
  results: [albumResult],
  total: 1,
  page: 1,
  per_page: SEARCH_PER_PAGE,
  total_pages: 1,
});

const musicianResults = apiResponse<SearchMusiciansResponseType>({
  query: "Casino",
  results: [musicianResult],
  total: 1,
  page: 1,
  per_page: SEARCH_PER_PAGE,
  total_pages: 1,
});

const trackResults = apiResponse<SearchTracksResponseType>({
  query: "Casino",
  results: [trackResult],
  total: 1,
  page: 1,
  per_page: SEARCH_PER_PAGE,
  total_pages: 1,
});

async function mockSearchApi(page: Page) {
  const requestedSearchRequests: string[] = [];
  const resultsByPath: Record<string, unknown> = {
    "/api/search": allResults,
    "/api/search/movies": movieResults,
    "/api/search/shows": showResults,
    "/api/search/albums": albumResults,
    "/api/search/musicians": musicianResults,
    "/api/search/tracks": trackResults,
  };

  const { unexpectedApiRequests } = await mockApi(page, {
    user: { is_admin: true },
    handle: async ({ route, url, method }) => {
      const results = resultsByPath[url.pathname];
      if (results) {
        requestedSearchRequests.push(`${url.pathname}${url.search}`);
        await fulfillJSON(route, results);
        return true;
      }

      if (url.pathname === "/api/music/tracks/liked-ids" && method === "GET") {
        await fulfillJSON(route, apiResponse({ liked_track_ids: [] }));
        return true;
      }

      return false;
    },
  });

  return {
    requestedSearchRequests,
    unexpectedApiRequests,
  };
}

test("search supports keyboard submission, tabs, and responsive layout", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  const { requestedSearchRequests, unexpectedApiRequests } =
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
  await expect(
    page.getByRole("link", {
      name: "Casino Original Soundtrack by Various Artists",
    }),
  ).toBeVisible();

  await page.getByRole("tab", { name: "Musicians" }).click();
  await expect(page).toHaveURL(/tab=musicians/);
  await expect(
    page.getByRole("link", {
      name: "Casino House Band, 2 albums, 18 tracks",
    }),
  ).toBeVisible();

  await page.getByRole("tab", { name: "Tracks" }).click();
  await expect(page).toHaveURL(/tab=tracks/);
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
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});
