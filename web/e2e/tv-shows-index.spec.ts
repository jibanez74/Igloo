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
import { SHOWS_PER_PAGE } from "../src/lib/constants";
import type { ShowLibraryItemType } from "../src/types";
import { apiResponse, fulfillJSON, pagedList } from "./e2e-api";
import { mockApi } from "./e2e-mock-api";
import { fillLibraryPage } from "./fixtures/movies";
import { libraryShow } from "./fixtures/shows";

function buildShowPage(
  featuredShows: ShowLibraryItemType[],
  prefix: string,
  startId: number,
) {
  return fillLibraryPage(
    featuredShows,
    { prefix, startId, perPage: SHOWS_PER_PAGE },
    libraryShow,
  );
}

const showsAllPath = "/tv-shows?tab=all&allPage=1&sort=asc&genresPage=1";
const showsGenresPath = "/tv-shows?tab=genres&allPage=1&sort=asc&genresPage=1";

const libraryPageOneShows = buildShowPage(
  [
    libraryShow(101, "Frost Harbor", 2024, "/frost-harbor.jpg"),
    libraryShow(102, "Quiet Channel", 2022),
  ],
  "Library Mock",
  1000,
);

const libraryPageTwoShows = [
  libraryShow(201, "Verdant Coast", 2025, "/verdant-coast.jpg"),
];

const dramaPageOneShows = buildShowPage(
  [
    libraryShow(301, "Sky Relay", 2025, "/sky-relay.jpg"),
    libraryShow(302, "Cinder Avenue", 2021),
  ],
  "Drama Mock",
  2000,
);

const dramaPageTwoShows = [
  libraryShow(325, "Afterglow", 2020, "/afterglow.jpg"),
];

const comedyPageOneShows = [
  libraryShow(401, "Quiet Channel", 2022),
];

const showGenres = [
  {
    genre_id: 10,
    genre_tag: "Drama",
    show_count: 25,
  },
  {
    genre_id: 20,
    genre_tag: "Comedy",
    show_count: 1,
  },
];

async function mockShowsApi(
  page: Page,
  requestedLibraryRequests: string[],
  requestedGenreRequests: string[],
) {
  const { unexpectedApiRequests } = await mockApi(page, {
    user: { is_admin: true },
    handle: async ({ route, url }) => {
      const secondPage = url.searchParams.get("page") === "2";

      if (url.pathname === "/api/shows/stats") {
        await fulfillJSON(route, apiResponse({ total_shows: 25 }));
        return true;
      }

      if (url.pathname === "/api/shows/library") {
        requestedLibraryRequests.push(`${url.pathname}${url.search}`);

        const shows = secondPage ? libraryPageTwoShows : libraryPageOneShows;
        await fulfillJSON(route, apiResponse(pagedList(url, "shows", shows, {
          total: 25,
          perPage: SHOWS_PER_PAGE,
        })));
        return true;
      }

      if (url.pathname === "/api/shows/genres") {
        await fulfillJSON(route, apiResponse({ genres: showGenres }));
        return true;
      }

      if (url.pathname === "/api/shows/genres/10/shows") {
        requestedGenreRequests.push(`${url.pathname}${url.search}`);

        const shows = secondPage ? dramaPageTwoShows : dramaPageOneShows;
        await fulfillJSON(route, apiResponse(pagedList(url, "shows", shows, {
          total: 25,
          perPage: SHOWS_PER_PAGE,
        })));
        return true;
      }

      if (url.pathname === "/api/shows/genres/20/shows") {
        requestedGenreRequests.push(`${url.pathname}${url.search}`);

        await fulfillJSON(route, apiResponse(pagedList(url, "shows", comedyPageOneShows, {
          total: 1,
          perPage: SHOWS_PER_PAGE,
        })));
        return true;
      }

      return false;
    },
  });

  return unexpectedApiRequests;
}

test("tv shows library shell and URL-backed tabs render accessibly", async ({ page }) => {
  const requestedLibraryRequests: string[] = [];
  const requestedGenreRequests: string[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockShowsApi(
    page,
    requestedLibraryRequests,
    requestedGenreRequests,
  );
  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto(showsAllPath);

  await expect(page).toHaveTitle("TV Shows - Igloo");
  await expect(page.getByRole("heading", { name: "TV Show Library", level: 1 })).toBeVisible();
  await expect(page.getByRole("region", { name: "Library statistics: 25 shows" })).toBeVisible();

  const tablist = page.getByRole("tablist");
  await expect(tablist).toBeVisible();
  await expect(page.getByRole("tab")).toHaveCount(2);

  const allShowsTab = page.getByRole("tab", { name: "All Shows" });
  const genresTab = page.getByRole("tab", { name: "Genres" });

  await expect(allShowsTab).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("tabpanel", { name: "All Shows" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Frost Harbor 2024", exact: true })).toBeVisible();

  await genresTab.click();

  await expect(page).toHaveURL(/tab=genres/);
  await expect(genresTab).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("tabpanel", { name: "Genres" })).toBeVisible();
  await expect(page.getByRole("list", { name: "TV show genres" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Drama 25 shows" })).toBeVisible();

  await allShowsTab.click();

  await expect(page).toHaveURL(/tab=all/);
  await expect(allShowsTab).toHaveAttribute("aria-selected", "true");
  await expect(page.getByRole("tabpanel", { name: "All Shows" })).toBeVisible();
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("all shows tab renders accessible show cards and URL-backed pagination", async ({ page }) => {
  const requestedLibraryRequests: string[] = [];
  const requestedGenreRequests: string[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockShowsApi(
    page,
    requestedLibraryRequests,
    requestedGenreRequests,
  );
  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto(showsAllPath);

  const frostHarborLink = page.getByRole("link", {
    name: "Frost Harbor 2024",
    exact: true,
  });
  const quietChannelLink = page.getByRole("link", {
    name: "Quiet Channel 2022",
    exact: true,
  });

  await expect(frostHarborLink).toBeVisible();
  await expect(quietChannelLink).toBeVisible();
  await expect(frostHarborLink).toHaveAttribute("href", "/tv-shows/101");
  await expect(quietChannelLink).toHaveAttribute("href", "/tv-shows/102");
  // A show has no single thing to play, so its card carries no play control.
  await expect(page.getByRole("link", { name: /^Play / })).toHaveCount(0);

  const posterBackedCard = frostHarborLink.locator("xpath=ancestor::article");
  const posterlessCard = quietChannelLink.locator("xpath=ancestor::article");

  await expect(posterBackedCard.locator("img")).toHaveAttribute(
    "src",
    /\/api\/tmdb\/images\/w500\/frost-harbor\.jpg$/,
  );
  await expect(posterlessCard.locator("img")).toHaveCount(0);

  await page.getByRole("button", { name: "Go to next page" }).click();

  await expect(page).toHaveURL(/allPage=2/);
  await expect
    .poll(() =>
      requestedLibraryRequests.some(requestPath => {
        const parsed = new URL(`http://localhost${requestPath}`);
        return (
          parsed.pathname === "/api/shows/library" &&
          parsed.searchParams.get("page") === "2" &&
          parsed.searchParams.get("per_page") === String(SHOWS_PER_PAGE) &&
          parsed.searchParams.get("sort") === "asc"
        );
      }),
    )
    .toBe(true);
  await expect(page.getByRole("link", { name: "Verdant Coast 2025", exact: true })).toBeVisible();

  // The grid does not order anything itself: it hands `sort` to the API and
  // renders what comes back. So the contract worth asserting is that the toggle
  // flips the URL, returns to page one, and reaches the server.
  await page
    .getByRole("button", { name: "Sorted A to Z, click to sort Z to A" })
    .click();

  await expect(page).toHaveURL(/sort=desc/);
  await expect(page).toHaveURL(/allPage=1/);
  await expect
    .poll(() =>
      requestedLibraryRequests.some(requestPath => {
        const parsed = new URL(`http://localhost${requestPath}`);
        return (
          parsed.pathname === "/api/shows/library" &&
          parsed.searchParams.get("page") === "1" &&
          parsed.searchParams.get("sort") === "desc"
        );
      }),
    )
    .toBe(true);
  await expect(
    page.getByRole("button", { name: "Sorted Z to A, click to sort A to Z" }),
  ).toBeVisible();

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("genres tab renders accessible counts, filtering, and URL-backed pagination", async ({ page }) => {
  const requestedLibraryRequests: string[] = [];
  const requestedGenreRequests: string[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockShowsApi(
    page,
    requestedLibraryRequests,
    requestedGenreRequests,
  );
  await page.setViewportSize(VIEWPORTS.desktop);
  await page.goto(showsGenresPath);

  const genresTab = page.getByRole("tab", { name: "Genres" });
  const genresList = page.getByRole("list", { name: "TV show genres" });
  const dramaButton = page.getByRole("button", { name: "Drama 25 shows" });

  await expect(genresTab).toHaveAttribute("aria-selected", "true");
  await expect(genresList).toBeVisible();
  await expect(dramaButton).toBeVisible();
  await expect(page.getByRole("button", { name: "Comedy 1 show" })).toBeVisible();

  await dramaButton.click();

  await expect(page).toHaveURL(/genreId=10/);
  await expect(page.getByText("Sky Relay")).toBeVisible();
  const clearGenreFilterButton = page.getByRole("button", {
    name: "Clear genre filter",
  });
  await expect(
    clearGenreFilterButton.locator(
      "xpath=preceding-sibling::span[contains(., '25 shows')]",
    ),
  ).toBeVisible();
  await expect(clearGenreFilterButton).toBeVisible();

  await page.getByRole("button", { name: "Go to next page" }).click();

  await expect(page).toHaveURL(/genresPage=2/);
  await expect
    .poll(() =>
      requestedGenreRequests.some(requestPath => {
        const parsed = new URL(`http://localhost${requestPath}`);
        return (
          parsed.pathname === "/api/shows/genres/10/shows" &&
          parsed.searchParams.get("page") === "2" &&
          parsed.searchParams.get("per_page") === String(SHOWS_PER_PAGE) &&
          parsed.searchParams.get("sort") === "asc"
        );
      }),
    )
    .toBe(true);
  await expect(page.getByRole("link", { name: "Afterglow 2020", exact: true })).toBeVisible();

  await page
    .getByRole("button", { name: "Sorted A to Z, click to sort Z to A" })
    .click();

  await expect(page).toHaveURL(/sort=desc/);
  await expect(page).toHaveURL(/genresPage=1/);
  await expect(page).toHaveURL(/genreId=10/);
  await expect
    .poll(() =>
      requestedGenreRequests.some(requestPath => {
        const parsed = new URL(`http://localhost${requestPath}`);
        return (
          parsed.pathname === "/api/shows/genres/10/shows" &&
          parsed.searchParams.get("page") === "1" &&
          parsed.searchParams.get("sort") === "desc"
        );
      }),
    )
    .toBe(true);

  await clearGenreFilterButton.click();

  await expect(page).not.toHaveURL(/genreId=/);
  await expect(page).not.toHaveURL(/genresPage=2/);
  await expect(genresList).toBeVisible();
  await expect(page.getByRole("link", { name: "Afterglow 2020", exact: true })).toHaveCount(0);
  await expect(dramaButton).toBeFocused();
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("tv shows tabs avoid horizontal overflow on mobile", async ({ page }) => {
  const requestedLibraryRequests: string[] = [];
  const requestedGenreRequests: string[] = [];
  const browserIssues = trackBrowserIssues(page);

  const unexpectedApiRequests = await mockShowsApi(
    page,
    requestedLibraryRequests,
    requestedGenreRequests,
  );
  await page.setViewportSize(VIEWPORTS.phone);
  await page.goto(showsAllPath);

  const tablist = page.getByRole("tablist");
  const stats = page.getByRole("region", { name: "Library statistics: 25 shows" });
  const allShowsLink = page.getByRole("link", {
    name: "Frost Harbor 2024",
    exact: true,
  });
  await expect(allShowsLink).toBeVisible();
  const allShowsCard = allShowsLink.locator("xpath=ancestor::article");
  const allShowsGrid = allShowsCard.locator("xpath=parent::*");

  await expectPageHasNoHorizontalScroll(page);
  await expectNoHorizontalOverflow(tablist, "tv shows tablist");
  await expectNoHorizontalOverflow(stats, "tv shows stats");
  await expectNoHorizontalOverflow(allShowsGrid, "all shows grid");
  await expectNoHorizontalOverflow(allShowsCard, "all shows card");
  await expectNoHorizontalOverflow(page.getByRole("navigation", { name: "pagination" }), "all shows pagination");

  await page.getByRole("tab", { name: "Genres" }).click();

  const genresList = page.getByRole("list", { name: "TV show genres" });
  const dramaButton = page.getByRole("button", { name: "Drama 25 shows" });
  await expect(dramaButton).toBeVisible();

  await expectPageHasNoHorizontalScroll(page);
  await expectNoHorizontalOverflow(tablist, "tv shows tablist");
  await expectNoHorizontalOverflow(stats, "tv shows stats");
  await expectNoHorizontalOverflow(genresList, "genres grid");
  await expectNoHorizontalOverflow(dramaButton, "genre button");

  await dramaButton.click();

  const selectedGenreCard = page
    .getByRole("link", { name: "Sky Relay 2025", exact: true })
    .locator("xpath=ancestor::article");
  const selectedGenreGrid = selectedGenreCard.locator("xpath=parent::*");
  const selectedGenreHeader = page
    .getByRole("button", { name: "Clear genre filter" })
    .locator("xpath=ancestor::div[contains(concat(' ', normalize-space(@class), ' '), ' mb-5 ')][1]");

  await expectPageHasNoHorizontalScroll(page);
  await expectNoHorizontalOverflow(selectedGenreHeader, "selected genre header");
  await expectNoHorizontalOverflow(selectedGenreGrid, "selected genre results");
  await expectNoHorizontalOverflow(selectedGenreCard, "selected genre card");
  await expectNoHorizontalOverflow(page.getByRole("navigation", { name: "pagination" }), "selected genre pagination");
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});
