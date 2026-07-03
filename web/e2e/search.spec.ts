import { expect, test, type Page, type Route } from "@playwright/test";
import { trackBrowserIssues } from "./e2e-browser-issues";
import {
  expectNoHorizontalOverflow,
  expectPageHasNoHorizontalScroll,
} from "./e2e-layout";
import type {
  SearchAlbumsResponseType,
  SearchAllResponseType,
  SearchMoviesResponseType,
  SearchMusiciansResponseType,
  SearchTracksResponseType,
} from "../src/types/search";
import { SEARCH_PER_PAGE } from "../src/lib/constants";

type NullableString = {
  String: string;
  Valid: boolean;
};

type NullableInt64 = {
  Int64: number;
  Valid: boolean;
};

type ApiResponse<T> = {
  error: false;
  data: T;
};

function nullableString(value = ""): NullableString {
  return {
    String: value,
    Valid: value.length > 0,
  };
}

function nullableInt64(value: number | null = null): NullableInt64 {
  return {
    Int64: value ?? 0,
    Valid: value != null,
  };
}

function apiResponse<T>(data: T): ApiResponse<T> {
  return {
    error: false,
    data,
  };
}

async function fulfillJSON(route: Route, body: unknown, status = 200) {
  await route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  });
}

const movieResult = {
  id: 7,
  title: "Casino Royale",
  poster_path: nullableString(),
  year: nullableInt64(2006),
  certification: nullableString("PG-13"),
} satisfies SearchAllResponseType["movies"]["results"][number];

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
  sort_name: "Casino House Band",
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
  file_path: "/music/casino-theme.flac",
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
  const unexpectedApiRequests: string[] = [];

  await page.route("**/api/**", async route => {
    const url = new URL(route.request().url());
    const method = route.request().method();

    if (url.pathname === "/api/auth/user") {
      await fulfillJSON(route, apiResponse({
        user: {
          id: 1,
          name: "Search User",
          email: "search@example.com",
          is_admin: true,
          avatar: null,
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-01T00:00:00Z",
        },
      }));
      return;
    }

    if (url.pathname === "/api/notifications/unread-count" && method === "GET") {
      await fulfillJSON(route, apiResponse({ unread_count: 0 }));
      return;
    }

    if (url.pathname === "/api/search") {
      requestedSearchRequests.push(`${url.pathname}${url.search}`);
      await fulfillJSON(route, allResults);
      return;
    }

    if (url.pathname === "/api/search/movies") {
      requestedSearchRequests.push(`${url.pathname}${url.search}`);
      await fulfillJSON(route, movieResults);
      return;
    }

    if (url.pathname === "/api/search/albums") {
      requestedSearchRequests.push(`${url.pathname}${url.search}`);
      await fulfillJSON(route, albumResults);
      return;
    }

    if (url.pathname === "/api/search/musicians") {
      requestedSearchRequests.push(`${url.pathname}${url.search}`);
      await fulfillJSON(route, musicianResults);
      return;
    }

    if (url.pathname === "/api/search/tracks") {
      requestedSearchRequests.push(`${url.pathname}${url.search}`);
      await fulfillJSON(route, trackResults);
      return;
    }

    const message = `Unexpected API request: ${method} ${url.pathname}${url.search}`;
    unexpectedApiRequests.push(message);
    await fulfillJSON(route, { error: true, message }, 500);
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

  await page.setViewportSize({ width: 1440, height: 900 });
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
  await expect(page.getByRole("tab")).toHaveCount(5);
  await expectPageHasNoHorizontalScroll(page);
  await expectNoHorizontalOverflow(
    page.getByRole("main"),
    "search page desktop main",
  );
  await expectNoHorizontalOverflow(tablist, "search tablist desktop");

  await page.getByRole("tab", { name: "Movies" }).click();
  await expect(page).toHaveURL(/tab=movies/);
  await expect(page.getByRole("tabpanel", { name: "Movies" })).toBeVisible();
  await expect(page.getByText("1 movies")).toBeVisible();

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

  await page.setViewportSize({ width: 390, height: 844 });
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
    `/api/search/albums?q=Casino&page=1&per_page=${SEARCH_PER_PAGE}`,
    `/api/search/musicians?q=Casino&page=1&per_page=${SEARCH_PER_PAGE}`,
    `/api/search/tracks?q=Casino&page=1&per_page=${SEARCH_PER_PAGE}`,
  ]);
  expect(unexpectedApiRequests).toEqual([]);
  browserIssues.assertClean();
});
