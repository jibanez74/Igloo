import { expect, test, type Page } from "@playwright/test";
import { MOVIES_PER_PAGE, SHOWS_PER_PAGE } from "../src/lib/constants";
import type {
  MovieGenreWithCountType,
  MoviesLibraryListItemType,
  ShowGenreWithCountType,
  ShowLibraryItemType,
} from "../src/types";
import { apiResponse, fulfillJSON, pagedList } from "./e2e-api";
import {
  assertMockSuiteClean,
  trackBrowserIssues,
} from "./e2e-browser-issues";
import { expectNoOverflowingElements, VIEWPORTS } from "./e2e-layout";
import { mockApi } from "./e2e-mock-api";
import {
  fillLibraryPage,
  libraryMovie,
  movieCardPreload,
} from "./fixtures/movies";
import { libraryShow, showCardPreload } from "./fixtures/shows";

// The movies and TV shows libraries share their tabs, grid, genre filter,
// pagination and sort (src/components/shared/Library*), so one table drives
// both. The movie-only playlists and liked views live in movies-index.spec.ts.

type LibraryTab = {
  name: string;
  param: string;
  /** Something only this tab's panel renders once its data arrives. */
  marker: { role: "button" | "link" | "list"; name: string };
};

type Card = { name: string; id: number };

type LibraryKind = {
  name: string;
  path: string;
  title: string;
  heading: string;
  statsLabel: string;
  tabs: LibraryTab[];
  genresList: string;
  /** The first chip is the 25-item genre the tests open; the second has one. */
  genreChips: [string, string];
  detailsHref: (id: number) => string;
  api: {
    library: string;
    genreItems: string;
    listKey: string;
    perPage: number;
  };
  /** The other GET responses (stats, genres, extras) keyed by pathname. */
  bodies: Record<string, unknown>;
  pages: { library: [unknown[], unknown[]]; genre: [unknown[], unknown[]] };
  /** What pointing at a card can request, or undefined for any other path. */
  cardPreload: (pathname: string) => unknown;
  cards: {
    posterBacked: Card & { poster: RegExp };
    posterless: Card;
    pageTwo: string;
    genrePageOne: string;
    genrePageTwo: string;
  };
};

const TOTAL = 25;
const GENRE_ID = 10;

function moviesKind(): LibraryKind {
  const fill = (featured: MoviesLibraryListItemType[], prefix: string, startId: number) =>
    fillLibraryPage(featured, { prefix, startId, perPage: MOVIES_PER_PAGE }, libraryMovie);

  const library: [MoviesLibraryListItemType[], MoviesLibraryListItemType[]] = [
    fill(
      [
        libraryMovie(101, "Signal Fire", 2024, "/signal-fire.jpg"),
        libraryMovie(102, "Quiet Harbor", 2022),
      ],
      "Library Mock",
      1000,
    ),
    [libraryMovie(201, "Verdant Run", 2025, "/verdant-run.jpg")],
  ];
  const genre: typeof library = [
    fill(
      [
        libraryMovie(301, "Sky Relay", 2025, "/sky-relay.jpg"),
        libraryMovie(302, "Cinder Avenue", 2021),
      ],
      "Action Mock",
      2000,
    ),
    [libraryMovie(325, "Afterburn", 2020, "/afterburn.jpg")],
  ];
  const byId = new Map([...library, ...genre].flat().map(movie => [movie.id, movie]));

  return {
    name: "movies",
    path: "/movies",
    title: "Movies - Igloo",
    heading: "Movie Library",
    statsLabel: `Library statistics: ${TOTAL} movies`,
    tabs: [
      { name: "All Movies", param: "all", marker: { role: "link", name: "Signal Fire 2024" } },
      { name: "Genres", param: "genres", marker: { role: "list", name: "Movie genres" } },
      { name: "Playlists", param: "playlists", marker: { role: "button", name: "New playlist" } },
    ],
    genresList: "Movie genres",
    genreChips: [`Action ${TOTAL} movies`, "Drama 1 movie"],
    detailsHref: id => `/movies/${id}`,
    api: {
      library: "/api/movies/library",
      genreItems: `/api/movies/genres/${GENRE_ID}/movies`,
      listKey: "movies",
      perPage: MOVIES_PER_PAGE,
    },
    bodies: {
      "/api/movies/stats": { total_movies: TOTAL },
      "/api/movies/genres": {
        genres: [
          { genre_id: GENRE_ID, genre_tag: "Action", movie_count: TOTAL },
          { genre_id: 20, genre_tag: "Drama", movie_count: 1 },
        ] satisfies MovieGenreWithCountType[],
      },
      "/api/movies/playlists": { playlists: [] },
      "/api/tmdb/status": { available: true },
    },
    pages: { library, genre },
    cardPreload: pathname => movieCardPreload(pathname, byId),
    cards: {
      posterBacked: { name: "Signal Fire 2024", id: 101, poster: /\/api\/tmdb\/images\/w500\/signal-fire\.jpg$/ },
      posterless: { name: "Quiet Harbor 2022", id: 102 },
      pageTwo: "Verdant Run 2025",
      genrePageOne: "Sky Relay 2025",
      genrePageTwo: "Afterburn 2020",
    },
  };
}

function showsKind(): LibraryKind {
  const fill = (featured: ShowLibraryItemType[], prefix: string, startId: number) =>
    fillLibraryPage(featured, { prefix, startId, perPage: SHOWS_PER_PAGE }, libraryShow);

  const library: [ShowLibraryItemType[], ShowLibraryItemType[]] = [
    fill(
      [
        libraryShow(101, "Frost Harbor", 2024, "/frost-harbor.jpg"),
        libraryShow(102, "Quiet Channel", 2022),
      ],
      "Library Mock",
      1000,
    ),
    [libraryShow(201, "Verdant Coast", 2025, "/verdant-coast.jpg")],
  ];
  const genre: typeof library = [
    fill(
      [
        libraryShow(301, "Sky Relay", 2025, "/sky-relay.jpg"),
        libraryShow(302, "Cinder Avenue", 2021),
      ],
      "Drama Mock",
      2000,
    ),
    [libraryShow(325, "Afterglow", 2020, "/afterglow.jpg")],
  ];
  const byId = new Map([...library, ...genre].flat().map(show => [show.id, show]));

  return {
    name: "TV shows",
    path: "/tv-shows",
    title: "TV Shows - Igloo",
    heading: "TV Show Library",
    statsLabel: `Library statistics: ${TOTAL} shows`,
    tabs: [
      { name: "All Shows", param: "all", marker: { role: "link", name: "Frost Harbor 2024" } },
      { name: "Genres", param: "genres", marker: { role: "list", name: "TV show genres" } },
    ],
    genresList: "TV show genres",
    genreChips: [`Drama ${TOTAL} shows`, "Comedy 1 show"],
    detailsHref: id => `/tv-shows/${id}`,
    api: {
      library: "/api/shows/library",
      genreItems: `/api/shows/genres/${GENRE_ID}/shows`,
      listKey: "shows",
      perPage: SHOWS_PER_PAGE,
    },
    bodies: {
      "/api/shows/stats": { total_shows: TOTAL },
      "/api/shows/genres": {
        genres: [
          { genre_id: GENRE_ID, genre_tag: "Drama", show_count: TOTAL },
          { genre_id: 20, genre_tag: "Comedy", show_count: 1 },
        ] satisfies ShowGenreWithCountType[],
      },
    },
    pages: { library, genre },
    cardPreload: pathname => showCardPreload(pathname, byId),
    cards: {
      posterBacked: { name: "Frost Harbor 2024", id: 101, poster: /\/api\/tmdb\/images\/w500\/frost-harbor\.jpg$/ },
      posterless: { name: "Quiet Channel 2022", id: 102 },
      pageTwo: "Verdant Coast 2025",
      genrePageOne: "Sky Relay 2025",
      genrePageTwo: "Afterglow 2020",
    },
  };
}

function tabPath(kind: LibraryKind, tab: string) {
  const playlistsPage = kind.tabs.some(t => t.param === "playlists") ? "&playlistsPage=1" : "";
  return `${kind.path}?tab=${tab}&allPage=1&sort=asc&genresPage=1${playlistsPage}`;
}

async function mockLibraryApi(page: Page, kind: LibraryKind) {
  const apiRequests: URL[] = [];

  const { unexpectedApiRequests } = await mockApi(page, {
    user: { is_admin: true },
    handle: async ({ route, url, method }) => {
      if (method !== "GET") {
        return false;
      }
      apiRequests.push(url);

      const pageIndex = url.searchParams.get("page") === "2" ? 1 : 0;
      const pageItems = {
        [kind.api.library]: kind.pages.library[pageIndex],
        [kind.api.genreItems]: kind.pages.genre[pageIndex],
      }[url.pathname];
      if (pageItems) {
        await fulfillJSON(route, apiResponse(pagedList(url, kind.api.listKey, pageItems, {
          total: TOTAL,
          perPage: kind.api.perPage,
        })));
        return true;
      }

      if (url.pathname in kind.bodies) {
        await fulfillJSON(route, apiResponse(kind.bodies[url.pathname]));
        return true;
      }

      const preload = kind.cardPreload(url.pathname);
      if (preload !== undefined) {
        await fulfillJSON(route, apiResponse(preload));
        return true;
      }

      return false;
    },
  });

  return { apiRequests, unexpectedApiRequests };
}

/** Waits for the page to have asked for `pathname` with `params` in its query. */
async function expectApiRequest(
  apiRequests: URL[],
  pathname: string,
  params: Record<string, string>,
) {
  await expect
    .poll(
      () =>
        apiRequests.some(
          url =>
            url.pathname === pathname &&
            Object.entries(params).every(([key, value]) => url.searchParams.get(key) === value),
        ),
      { message: `expected a request for ${pathname} with ${JSON.stringify(params)}` },
    )
    .toBe(true);
}

async function expectTabShown(page: Page, tab: LibraryTab) {
  await expect(page).toHaveURL(new RegExp(`tab=${tab.param}`));
  await expect(page.getByRole("tab", { name: tab.name })).toHaveAttribute("aria-selected", "true");

  const panel = page.getByRole("tabpanel", { name: tab.name });
  await expect(panel).toBeVisible();
  await expect(
    panel.getByRole(tab.marker.role, { name: tab.marker.name, exact: true }),
  ).toBeVisible();
}

function cardFor(page: Page, name: string) {
  return page.getByRole("article").filter({
    has: page.getByRole("link", { name, exact: true }),
  });
}

for (const kind of [moviesKind(), showsKind()]) {
  test.describe(`${kind.name} library`, () => {
    test("shell and URL-backed tabs render accessibly", async ({ page }) => {
      const browserIssues = trackBrowserIssues(page);
      const { unexpectedApiRequests } = await mockLibraryApi(page, kind);

      await page.setViewportSize(VIEWPORTS.desktop);
      await page.goto(tabPath(kind, "all"));

      await expect(page).toHaveTitle(kind.title);
      await expect(page.getByRole("heading", { name: kind.heading, level: 1 })).toBeVisible();
      await expect(page.getByRole("region", { name: kind.statsLabel })).toBeVisible();
      await expect(page.getByRole("tab")).toHaveCount(kind.tabs.length);

      const [firstTab, ...otherTabs] = kind.tabs;
      await expectTabShown(page, firstTab);
      for (const tab of [...otherTabs, firstTab]) {
        await page.getByRole("tab", { name: tab.name }).click();
        await expectTabShown(page, tab);
      }

      assertMockSuiteClean(browserIssues, unexpectedApiRequests);
    });

    test("all tab renders cards and URL-backed pagination and sort", async ({ page }) => {
      const browserIssues = trackBrowserIssues(page);
      const { apiRequests, unexpectedApiRequests } = await mockLibraryApi(page, kind);

      await page.setViewportSize(VIEWPORTS.desktop);
      await page.goto(tabPath(kind, "all"));

      const { posterBacked, posterless } = kind.cards;
      await expect(page.getByRole("link", { name: posterBacked.name, exact: true })).toHaveAttribute(
        "href",
        kind.detailsHref(posterBacked.id),
      );
      await expect(page.getByRole("link", { name: posterless.name, exact: true })).toHaveAttribute(
        "href",
        kind.detailsHref(posterless.id),
      );
      await expect(cardFor(page, posterBacked.name).locator("img")).toHaveAttribute(
        "src",
        posterBacked.poster,
      );
      await expect(cardFor(page, posterless.name).locator("img")).toHaveCount(0);

      await page.getByRole("button", { name: "Go to next page" }).click();

      await expect(page).toHaveURL(/allPage=2/);
      await expectApiRequest(apiRequests, kind.api.library, {
        page: "2",
        per_page: String(kind.api.perPage),
        sort: "asc",
      });
      await expect(page.getByRole("link", { name: kind.cards.pageTwo, exact: true })).toBeVisible();

      // The grid does not order anything itself: it hands `sort` to the API
      // and renders what comes back. So the contract is that the toggle flips
      // the URL, returns to page one, and reaches the server.
      await page.getByRole("button", { name: "Sorted A to Z, click to sort Z to A" }).click();

      await expect(page).toHaveURL(/sort=desc/);
      await expect(page).toHaveURL(/allPage=1/);
      await expectApiRequest(apiRequests, kind.api.library, { page: "1", sort: "desc" });
      await expect(
        page.getByRole("button", { name: "Sorted Z to A, click to sort A to Z" }),
      ).toBeVisible();

      assertMockSuiteClean(browserIssues, unexpectedApiRequests);
    });

    test("genres tab filters, paginates, sorts and clears through the URL", async ({ page }) => {
      const browserIssues = trackBrowserIssues(page);
      const { apiRequests, unexpectedApiRequests } = await mockLibraryApi(page, kind);

      await page.setViewportSize(VIEWPORTS.desktop);
      await page.goto(tabPath(kind, "genres"));

      const genresList = page.getByRole("list", { name: kind.genresList });
      const [genreChipName, singleChipName] = kind.genreChips;
      const genreChip = page.getByRole("button", { name: genreChipName });
      const nextPage = page.getByRole("button", { name: "Go to next page" });
      const genrePageTwoCard = page.getByRole("link", { name: kind.cards.genrePageTwo, exact: true });

      await expect(page.getByRole("tab", { name: "Genres" })).toHaveAttribute("aria-selected", "true");
      await expect(genresList).toBeVisible();
      await expect(genreChip).toBeVisible();
      await expect(page.getByRole("button", { name: singleChipName })).toBeVisible();

      await genreChip.click();

      await expect(page).toHaveURL(new RegExp(`genreId=${GENRE_ID}`));
      await expect(page.getByRole("link", { name: kind.cards.genrePageOne, exact: true })).toBeVisible();

      await nextPage.click();

      await expect(page).toHaveURL(/genresPage=2/);
      await expectApiRequest(apiRequests, kind.api.genreItems, {
        page: "2",
        per_page: String(kind.api.perPage),
        sort: "asc",
      });
      await expect(genrePageTwoCard).toBeVisible();

      await page.getByRole("button", { name: "Sorted A to Z, click to sort Z to A" }).click();

      await expect(page).toHaveURL(/sort=desc/);
      await expect(page).toHaveURL(/genresPage=1/);
      await expect(page).toHaveURL(new RegExp(`genreId=${GENRE_ID}`));
      await expectApiRequest(apiRequests, kind.api.genreItems, { page: "1", sort: "desc" });

      // Clearing from a later page must drop the page along with the genre.
      await nextPage.click();
      await expect(page).toHaveURL(/genresPage=2/);
      await expect(genrePageTwoCard).toBeVisible();

      await page.getByRole("button", { name: "Clear genre filter" }).click();

      await expect(page).not.toHaveURL(/genreId=/);
      await expect(page).not.toHaveURL(/genresPage=2/);
      await expect(genresList).toBeVisible();
      await expect(genrePageTwoCard).toHaveCount(0);
      await expect(genreChip).toBeFocused();

      assertMockSuiteClean(browserIssues, unexpectedApiRequests);
    });

    test("tabs avoid horizontal overflow on a phone", async ({ page }) => {
      const browserIssues = trackBrowserIssues(page);
      const { unexpectedApiRequests } = await mockLibraryApi(page, kind);

      await page.setViewportSize(VIEWPORTS.phone);
      await page.goto(tabPath(kind, "all"));

      await expect(page.getByRole("navigation", { name: "pagination" })).toBeVisible();
      await expect(
        page.getByRole("link", { name: kind.cards.posterBacked.name, exact: true }),
      ).toBeVisible();
      await expectNoOverflowingElements(page);

      await page.getByRole("tab", { name: "Genres" }).click();
      await expect(page.getByRole("button", { name: kind.genreChips[0] })).toBeVisible();
      await expectNoOverflowingElements(page);

      await page.getByRole("button", { name: kind.genreChips[0] }).click();
      await expect(page.getByRole("link", { name: kind.cards.genrePageOne, exact: true })).toBeVisible();
      await expect(page.getByRole("navigation", { name: "pagination" })).toBeVisible();
      await expectNoOverflowingElements(page);

      assertMockSuiteClean(browserIssues, unexpectedApiRequests);
    });
  });
}
