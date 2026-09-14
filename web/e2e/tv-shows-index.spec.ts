import { expect, test, type Page } from "@playwright/test";
import {
  assertMockSuiteClean,
  trackBrowserIssues,
} from "./e2e-browser-issues";
import {
  expectNoHorizontalOverflow,
  expectPageHasNoHorizontalScroll,
} from "./e2e-layout";
import { SHOWS_PER_PAGE } from "../src/lib/constants";
import {
  fulfillJSON,
  nullableInt64,
  nullableString,
  fulfillIdleScanStatus,
} from "./e2e-api";

function apiResponse(data: unknown) {
  return {
    error: false,
    data,
  };
}

function show(id: number, name: string, year: number, posterPath = "") {
  return {
    id,
    name,
    poster_path: nullableString(posterPath),
    premiere_year: nullableInt64(year),
    certification: nullableString("TV-14"),
  };
}

function buildShowPage(
  featuredShows: ReturnType<typeof show>[],
  fillerPrefix: string,
  fillerStartId: number,
) {
  return [
    ...featuredShows,
    ...Array.from(
      { length: SHOWS_PER_PAGE - featuredShows.length },
      (_, index) =>
        show(
          fillerStartId + index,
          `${fillerPrefix} ${index + 1}`,
          2000 + ((index + 1) % 20),
          index % 2 === 0
            ? `/${fillerPrefix.toLowerCase().replaceAll(" ", "-")}-${index + 1}.jpg`
            : "",
        ),
    ),
  ];
}

const showsAllPath = "/tv-shows?tab=all&allPage=1&sort=asc&genresPage=1";
const showsGenresPath = "/tv-shows?tab=genres&allPage=1&sort=asc&genresPage=1";

const libraryPageOneShows = buildShowPage(
  [
    show(101, "Frost Harbor", 2024, "/frost-harbor.jpg"),
    show(102, "Quiet Channel", 2022),
  ],
  "Library Mock",
  1000,
);

const libraryPageTwoShows = [
  show(201, "Verdant Coast", 2025, "/verdant-coast.jpg"),
];

const dramaPageOneShows = buildShowPage(
  [
    show(301, "Sky Relay", 2025, "/sky-relay.jpg"),
    show(302, "Cinder Avenue", 2021),
  ],
  "Drama Mock",
  2000,
);

const dramaPageTwoShows = [
  show(325, "Afterglow", 2020, "/afterglow.jpg"),
];

const comedyPageOneShows = [
  show(401, "Quiet Channel", 2022),
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
  const unexpectedApiRequests: string[] = [];

  await page.route("**/api/**", async route => {
    const url = new URL(route.request().url());
    const method = route.request().method();

    if (await fulfillIdleScanStatus(route, url.pathname)) {
      return;
    }

    if (url.pathname.startsWith("/api/tmdb/images/")) {
      await route.fulfill({
        status: 200,
        contentType: "image/svg+xml",
        body: `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="150" viewBox="0 0 100 150"><rect width="100" height="150" fill="#0ea5e9"/><rect x="12" y="12" width="76" height="126" rx="10" fill="#0f172a"/><circle cx="50" cy="50" r="18" fill="#f8fafc"/><rect x="24" y="92" width="52" height="10" rx="5" fill="#f8fafc"/><rect x="30" y="110" width="40" height="8" rx="4" fill="#7dd3fc"/></svg>`,
      });
      return;
    }

    if (url.pathname === "/api/auth/user") {
      await fulfillJSON(route, apiResponse({
        user: {
          id: 1,
          name: "Shows User",
          email: "shows@example.com",
          is_admin: true,
          avatar: null,
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-01T00:00:00Z",
        },
      }));
      return;
    }

    if (url.pathname === "/api/notifications/unread-count") {
      await fulfillJSON(route, apiResponse({ unread_count: 0 }));
      return;
    }

    if (url.pathname === "/api/shows/stats") {
      await fulfillJSON(route, apiResponse({ total_shows: 25 }));
      return;
    }

    if (url.pathname === "/api/shows/library") {
      const libraryPage = Number(url.searchParams.get("page") ?? "1");
      const perPage = Number(
        url.searchParams.get("per_page") ?? String(SHOWS_PER_PAGE),
      );
      const sort = url.searchParams.get("sort") === "desc" ? "desc" : "asc";
      requestedLibraryRequests.push(`${url.pathname}${url.search}`);

      await fulfillJSON(route, apiResponse({
        shows: libraryPage === 2 ? libraryPageTwoShows : libraryPageOneShows,
        total: 25,
        page: libraryPage,
        per_page: perPage,
        total_pages: 2,
        sort,
      }));
      return;
    }

    if (url.pathname === "/api/shows/genres") {
      await fulfillJSON(route, apiResponse({ genres: showGenres }));
      return;
    }

    if (url.pathname === "/api/shows/genres/10/shows") {
      const genrePage = Number(url.searchParams.get("page") ?? "1");
      const perPage = Number(
        url.searchParams.get("per_page") ?? String(SHOWS_PER_PAGE),
      );
      const sort = url.searchParams.get("sort") === "desc" ? "desc" : "asc";
      requestedGenreRequests.push(`${url.pathname}${url.search}`);

      await fulfillJSON(route, apiResponse({
        shows: genrePage === 2 ? dramaPageTwoShows : dramaPageOneShows,
        total: 25,
        page: genrePage,
        per_page: perPage,
        total_pages: 2,
        sort,
      }));
      return;
    }

    if (url.pathname === "/api/shows/genres/20/shows") {
      const genrePage = Number(url.searchParams.get("page") ?? "1");
      const perPage = Number(
        url.searchParams.get("per_page") ?? String(SHOWS_PER_PAGE),
      );
      const sort = url.searchParams.get("sort") === "desc" ? "desc" : "asc";
      requestedGenreRequests.push(`${url.pathname}${url.search}`);

      await fulfillJSON(route, apiResponse({
        shows: comedyPageOneShows,
        total: 1,
        page: genrePage,
        per_page: perPage,
        total_pages: 1,
        sort,
      }));
      return;
    }

    const message = `Unexpected API request: ${method} ${url.pathname}${url.search}`;
    unexpectedApiRequests.push(message);
    await fulfillJSON(route, { error: true, message }, 500);
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
  await page.setViewportSize({ width: 1440, height: 900 });
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
  await page.setViewportSize({ width: 1440, height: 900 });
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
  await page.setViewportSize({ width: 1440, height: 900 });
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
  await page.setViewportSize({ width: 390, height: 844 });
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
