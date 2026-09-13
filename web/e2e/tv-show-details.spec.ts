import { expect, test, type Page } from "@playwright/test";
import {
  assertMockSuiteClean,
  trackBrowserIssues,
} from "./e2e-browser-issues";
import { expectPageHasNoHorizontalScroll } from "./e2e-layout";
import {
  fulfillIdleScanStatus,
  fulfillJSON,
  nullableFloat64,
  nullableInt64,
  nullableString,
} from "./e2e-api";

function apiResponse(data: unknown) {
  return { error: false, data };
}

const showId = 401;

// Specials last, matching GetShowSeasonSummaries. Season 2 is partial on
// purpose, so the availability chips have something to say.
const seasons = [
  {
    id: 5001,
    season_number: 1,
    name: "Season 1",
    overview: nullableString("The harbor freezes."),
    air_date: nullableString("2026-03-01"),
    poster_path: nullableString("/frost-harbor.jpg"),
    tmdb_episode_count: nullableInt64(2),
    available_episode_count: 2,
  },
  {
    id: 5002,
    season_number: 2,
    name: "Season 2",
    overview: nullableString("The thaw begins."),
    air_date: nullableString("2026-09-01"),
    poster_path: nullableString("/frost-harbor.jpg"),
    tmdb_episode_count: nullableInt64(8),
    available_episode_count: 1,
  },
  {
    id: 5000,
    season_number: 0,
    name: "Specials",
    overview: nullableString("Behind the ice."),
    air_date: nullableString("2026-02-01"),
    poster_path: nullableString("/frost-harbor.jpg"),
    tmdb_episode_count: nullableInt64(1),
    available_episode_count: 1,
  },
];

function episode(seasonNumber: number, episodeNumber: number) {
  return {
    id: 70000 + seasonNumber * 100 + episodeNumber,
    episode_number: episodeNumber,
    name: `Season ${seasonNumber} Episode ${episodeNumber}`,
    overview: nullableString("Something happens in the harbor."),
    air_date: nullableString("2026-03-08"),
    still_path: nullableString("/still.jpg"),
    tmdb_runtime: nullableInt64(47),
    vote_average: nullableFloat64(8.1),
    vote_count: nullableInt64(220),
  };
}

const showDetailsPayload = {
  show: {
    id: showId,
    name: "Frost Harbor",
    original_name: nullableString("Frost Harbor"),
    premiere_year: nullableInt64(2026),
    tmdb_id: nullableInt64(90210),
    overview: nullableString(
      "A harbor freezes over and the town changes with it.",
    ),
    tagline: nullableString("The ice remembers."),
    language: nullableString("en"),
    origin_countries: nullableString("US"),
    first_air_date: nullableString("2026-03-01"),
    last_air_date: nullableString("2026-11-20"),
    status: nullableString("Returning Series"),
    type: nullableString("Scripted"),
    poster_path: nullableString("/frost-harbor.jpg"),
    backdrop_path: nullableString("/frost-harbor-backdrop.jpg"),
    vote_average: nullableFloat64(8.4),
    vote_count: nullableInt64(1200),
    certification: nullableString("TV-14"),
    tmdb_season_count: nullableInt64(2),
    tmdb_episode_count: nullableInt64(10),
  },
  seasons,
  cast: [
    {
      credit_id: "credit-lead-1",
      artist_id: 900,
      character: "Harbor Master",
      cast_order: 0,
      episode_count: 10,
      artist_name: "Ada Frost",
      artist_profile: nullableString("/ada.jpg"),
    },
    {
      // Same artist, second role: the rows are keyed on credit_id.
      credit_id: "credit-lead-2",
      artist_id: 900,
      character: "The Stranger",
      cast_order: 1,
      episode_count: 2,
      artist_name: "Ada Frost",
      artist_profile: nullableString("/ada.jpg"),
    },
  ],
  crew: [
    {
      credit_id: "crew-1",
      artist_id: 901,
      department: "Directing",
      job: "Director",
      episode_count: 6,
      artist_name: "Bo Winter",
      artist_profile: nullableString(""),
    },
  ],
  creators: [{ id: 900, name: "Ada Frost", profile: nullableString("/ada.jpg") }],
  genres: [{ id: 1, tag: "Drama" }],
  networks: [
    {
      id: 77,
      name: "Glacier Network",
      logo: nullableString("/network.png"),
      country: nullableString("US"),
    },
  ],
  production_companies: [{ id: 88, name: "Glacier Pictures" }],
  extra_videos: [
    {
      id: 1,
      title: "Frost Harbor Trailer",
      key: "frost-harbor-trailer",
      type: "trailer",
      site: "youtube",
    },
  ],
};

function seasonEpisodesPayload(seasonNumber: number) {
  const season = seasons.find(s => s.season_number === seasonNumber);
  if (!season) return null;

  return {
    season,
    episodes: Array.from({ length: season.available_episode_count }, (_, i) =>
      episode(seasonNumber, i + 1),
    ),
  };
}

async function mockShowDetailsApi(page: Page) {
  const unexpectedApiRequests: string[] = [];

  await page.route("**/api/**", async route => {
    const url = new URL(route.request().url());
    const method = route.request().method();

    if (
      url.pathname.startsWith("/api/tmdb/images/") ||
      url.pathname.startsWith("/api/youtube/thumbnails/")
    ) {
      await route.fulfill({
        status: 200,
        contentType: "image/svg+xml",
        body: `<svg xmlns="http://www.w3.org/2000/svg" width="160" height="240" viewBox="0 0 160 240"><rect width="160" height="240" fill="#0f172a"/></svg>`,
      });
      return;
    }

    // The app shell polls every scan-status endpoint for admins on every route.
    if (await fulfillIdleScanStatus(route, url.pathname)) {
      return;
    }

    if (method !== "GET") {
      const message = `Unexpected API request: ${method} ${url.pathname}${url.search}`;
      unexpectedApiRequests.push(message);
      await fulfillJSON(route, { error: true, message }, 405);
      return;
    }

    if (url.pathname === "/api/auth/user") {
      await fulfillJSON(
        route,
        apiResponse({
          user: {
            id: 1,
            name: "Show User",
            email: "shows@example.com",
            is_admin: false,
            avatar: null,
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z",
          },
        }),
      );
      return;
    }

    if (url.pathname === "/api/notifications/unread-count") {
      await fulfillJSON(route, apiResponse({ unread_count: 0 }));
      return;
    }

    if (url.pathname === `/api/shows/details/${showId}`) {
      await fulfillJSON(route, apiResponse(showDetailsPayload));
      return;
    }

    const episodesMatch = url.pathname.match(
      /^\/api\/shows\/(\d+)\/seasons\/(\d+)\/episodes$/,
    );
    if (episodesMatch) {
      const payload = seasonEpisodesPayload(Number(episodesMatch[2]));
      if (payload === null) {
        await fulfillJSON(route, { error: true, message: "season not found" }, 404);
        return;
      }

      await fulfillJSON(route, apiResponse(payload));
      return;
    }

    const message = `Unexpected API request: ${method} ${url.pathname}${url.search}`;
    unexpectedApiRequests.push(message);
    await fulfillJSON(route, { error: true, message }, 500);
  });

  return unexpectedApiRequests;
}

test("show details renders hero, seasons, and credits without console issues", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockShowDetailsApi(page);

  await page.setViewportSize({ width: 1440, height: 1200 });
  await page.goto(`/tv-shows/${showId}`);

  await expect(
    page.getByRole("heading", { level: 1, name: /Frost Harbor/ }),
  ).toBeVisible();

  // Availability is stated in words, not by color alone.
  await expect(page.getByText("2 seasons in this library")).toBeVisible();
  await expect(
    page.getByText("3 of 10 episodes available in this library"),
  ).toBeVisible();

  // Specials sort last, so season 1 is selected by default.
  await expect(page.getByRole("tab", { name: "Season 1" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await expect(
    page.getByRole("list", { name: /Season 1 episodes, 2 in this library/ }),
  ).toBeVisible();

  await expect(
    page.getByRole("heading", { name: "About Frost Harbor" }),
  ).toBeVisible();
  await expect(page.getByText("Glacier Network")).toBeVisible();

  // Both roles of one artist render; keying on credit_id keeps them distinct.
  await expect(
    page.getByRole("article", { name: /Ada Frost as Harbor Master, 10 episodes/ }),
  ).toBeVisible();
  await expect(
    page.getByRole("article", { name: /Ada Frost as The Stranger, 2 episodes/ }),
  ).toBeVisible();

  await expectPageHasNoHorizontalScroll(page);
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("selecting a season puts it in the URL and loads its episodes", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockShowDetailsApi(page);

  await page.goto(`/tv-shows/${showId}`);

  await expect(
    page.getByRole("heading", { level: 1, name: /Frost Harbor/ }),
  ).toBeVisible();

  await page.getByRole("tab", { name: "Specials" }).click();

  await expect(page).toHaveURL(new RegExp(`/tv-shows/${showId}\\?season=0$`));
  await expect(
    page.getByRole("list", { name: /Specials episodes, 1 in this library/ }),
  ).toBeVisible();

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("the season in the URL is what the page opens on", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockShowDetailsApi(page);

  await page.goto(`/tv-shows/${showId}?season=2`);

  await expect(page.getByRole("tab", { name: "Season 2" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await expect(
    page.getByRole("list", { name: /Season 2 episodes, 1 in this library/ }),
  ).toBeVisible();

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("season tabs are reachable and operable from the keyboard", async ({
  page,
}) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockShowDetailsApi(page);

  await page.goto(`/tv-shows/${showId}`);

  const season1 = page.getByRole("tab", { name: "Season 1" });
  await expect(season1).toBeVisible();

  await season1.focus();
  await expect(season1).toBeFocused();

  // Radix tabs move selection with the arrow keys.
  await page.keyboard.press("ArrowRight");
  await expect(page.getByRole("tab", { name: "Season 2" })).toBeFocused();
  await expect(
    page.getByRole("list", { name: /Season 2 episodes/ }),
  ).toBeVisible();

  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});

test("show details holds its layout at phone width", async ({ page }) => {
  const browserIssues = trackBrowserIssues(page);
  const unexpectedApiRequests = await mockShowDetailsApi(page);

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`/tv-shows/${showId}`);

  await expect(
    page.getByRole("heading", { level: 1, name: /Frost Harbor/ }),
  ).toBeVisible();
  await expect(
    page.getByRole("list", { name: /Season 1 episodes/ }),
  ).toBeVisible();

  await expectPageHasNoHorizontalScroll(page);
  assertMockSuiteClean(browserIssues, unexpectedApiRequests);
});
