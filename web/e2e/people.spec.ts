import { expect, test, type Page } from "@playwright/test";
import { mkdir } from "node:fs/promises";
import { personProfile, personTitle } from "../src/test/helpers/people";
import { seasonEpisodes, showDetails } from "../src/test/helpers/show-details";
import { playbackSettings } from "../src/test/helpers/fixtures";
import { apiResponse, fulfillJSON, pagedList, nullableFloat64, nullableInt64, nullableString } from "./e2e-api";
import { mockApi } from "./e2e-mock-api";
import { assertMockSuiteClean, trackBrowserIssues } from "./e2e-browser-issues";
import { expectPageHasNoHorizontalScroll, tabTo, VIEWPORTS } from "./e2e-layout";
import type { PersonData } from "../src/types/people";
import type { LibraryMovieDetailsResponse } from "../src/types";

const movie: LibraryMovieDetailsResponse = {
  movie: { id: 711, title: "Library Movie", adult: false, tmdb_id: nullableInt64(10), imdb_id: nullableString(), poster_path: nullableString(), backdrop_path: nullableString(), language: nullableString("en"), year: nullableInt64(2024), release_date: nullableString("2024-01-01"), overview: nullableString("Movie overview."), tag_line: nullableString(), certification: nullableString(), critic_rating: nullableFloat64(), audience_rating: nullableFloat64(), revenue: nullableFloat64(), budget: nullableFloat64(), run_time: nullableInt64(100), duration: nullableFloat64(6000) },
  cast: [{ id: 1, tmdb_id: 9001, character: "Hero", cast_order: 0, artist_name: "Alex Vega", artist_profile: nullableString() }],
  crew: [], genres: [], production_companies: [], extra_videos: [],
};

async function mockPeople(page: Page, profile: PersonData | (() => PersonData) = personProfile()) {
  return mockApi(page, { handle: async ({ route, url }) => {
    const bodies: Record<string, unknown> = {
      "/api/movies/stats": { total_movies: 0 },
      "/api/movies/library": pagedList(url, "movies", [], { total: 0, perPage: 24 }),
      "/api/shows/stats": { total_shows: 0, total_seasons: 0, total_episodes: 0 },
      "/api/shows/library": pagedList(url, "shows", [], { total: 0, perPage: 24 }),
      "/api/music/stats": { total_albums: 0, total_tracks: 0, total_musicians: 0, total_duration: 0 },
      "/api/music/albums": pagedList(url, "albums", [], { total: 0, perPage: 24 }),
      "/api/people/9001": typeof profile === "function" ? profile() : profile,
      "/api/shows/details/401": showDetails(),
      "/api/shows/401/seasons/1/episodes": seasonEpisodes(1),
      "/api/shows/401/seasons/2/episodes": seasonEpisodes(2),
      "/api/movies/details/711": movie,
      "/api/movies/711/technical-details": { movie: { file_name: "movie.mkv", size: 1, container: "mkv", mime_type: "video/x-matroska", run_time: nullableInt64(100), duration: nullableFloat64(6000) }, video_streams: [], audio_streams: [], subtitles: [], chapters: [] },
      "/api/movies/711/like-status": { is_liked: false },
      "/api/movies/711/watch-progress": { progress_sec: 0, duration_sec: 6000, watched: false },
      "/api/settings/playback": { settings: playbackSettings() },
    };
    if (!(url.pathname in bodies)) return false;
    await fulfillJSON(route, apiResponse(bodies[url.pathname]));
    return true;
  } });
}

test("movie to person by keyboard, then to a library title", async ({ page }) => {
  const issues = trackBrowserIssues(page);
  const { unexpectedApiRequests } = await mockPeople(page);
  await page.goto("/movies/711");
  const actor = page.getByRole("link", { name: "Alex Vega as Hero" });
  await tabTo(page, actor);
  await expect(actor).toBeFocused();
  expect(await actor.evaluate(element => getComputedStyle(element).boxShadow)).not.toBe("none");
  await page.keyboard.press("Enter");
  await expect(page.getByRole("heading", { level: 1, name: "Alex Vega" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Back", exact: true })).toHaveAttribute("href", "/movies/711");
  await page.getByRole("region", { name: "Filmography" }).getByRole("link", { name: /Library Show/ }).click();
  await expect(page).toHaveURL(/\/tv-shows\/401$/);
  await expect(page.getByRole("heading", { level: 1, name: /Frost Harbor/ })).toBeVisible();
  assertMockSuiteClean(issues, unexpectedApiRequests);
});

test("show to person preserves season return navigation", async ({ page }) => {
  const issues = trackBrowserIssues(page);
  const { unexpectedApiRequests } = await mockPeople(page);
  await page.goto("/tv-shows/401?season=2");
  await page.getByRole("link", { name: /Ada Contract as Captain, 10 episodes/ }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Alex Vega" })).toBeVisible();
  await page.getByRole("link", { name: "Back", exact: true }).click();
  await expect(page).toHaveURL(/\/tv-shows\/401\?season=2$/);
  await expect(page.getByRole("tab", { name: "Season 2" })).toHaveAttribute("aria-selected", "true");
  assertMockSuiteClean(issues, unexpectedApiRequests);
});

test("partial profile keeps local credits usable and retries enrichment", async ({ page }) => {
  const issues = trackBrowserIssues(page);
  let available = false;
  const { unexpectedApiRequests } = await mockPeople(page, () => personProfile({ enrichment_status: available ? "available" : "unavailable" }));
  await page.goto("/people/9001");
  await expect(page.getByText(/TMDB is temporarily unavailable/)).toBeVisible();
  await expect(page.getByRole("region", { name: "Filmography" }).getByRole("link", { name: /Library Movie/ })).toBeVisible();
  available = true;
  await page.getByRole("button", { name: "Try again" }).click();
  await expect(page.getByText(/TMDB is temporarily unavailable/)).toHaveCount(0);
  await page.getByRole("region", { name: "Filmography" }).getByRole("link", { name: /Library Movie/ }).click();
  await expect(page.getByRole("heading", { level: 1, name: /Library Movie/ })).toBeVisible();
  assertMockSuiteClean(issues, unexpectedApiRequests);
});

for (const theme of ["light", "dark"]) {
  for (const [viewportName, viewport] of Object.entries({ phone: VIEWPORTS.phone, desktop: VIEWPORTS.desktop })) {
    test(`person layout and keyboard disclosure at ${viewportName} in ${theme}`, async ({ page }) => {
      const issues = trackBrowserIssues(page);
      await page.addInitScript(value => localStorage.setItem("igloo-theme", value), theme);
      await page.setViewportSize(viewport);
      await page.emulateMedia({ reducedMotion: "reduce" });
      const profile = personProfile({ filmography: Array.from({ length: 75 }, (_, i) => personTitle(i + 1, { title: `Career ${String(i).padStart(2, "0")}` })) });
      profile.person.name = "Alexandra Vega With An Exceptionally Long Professional Name";
      profile.person.biography = `${"A long career biography with many details. ".repeat(40)}\n\nFinal biography paragraph.`;
      if (viewportName === "phone") profile.person.profile_path = "";
      const { unexpectedApiRequests } = await mockPeople(page, profile);
      await page.goto("/people/9001");
      await expect(page.getByRole("heading", { level: 1, name: profile.person.name })).toBeVisible();
      await expectPageHasNoHorizontalScroll(page);
      await page.keyboard.press("Tab");
      const biographySkip = page.getByRole("link", { name: "Skip to biography" });
      await tabTo(page, biographySkip);
      await page.keyboard.press("Enter");
      await expect(page.getByRole("heading", { name: "Biography", exact: true })).toBeFocused();
      const more = page.getByRole("button", { name: /^(Read more|Show less)$/ });
      await tabTo(page, more);
      await page.keyboard.press("Enter");
      await expect(more).toHaveAttribute("aria-expanded", "true");
      await expect(page.getByText("Final biography paragraph.")).toBeVisible();
      await page.keyboard.press("Space");
      await expect(more).toHaveAttribute("aria-expanded", "false");
      await page.getByLabel("Media type").selectOption("tv");
      await expect(page.getByText("No credits match these filters.")).toBeVisible();
      await expectPageHasNoHorizontalScroll(page);
      await page.getByLabel("Media type").selectOption("all");
      await page.evaluate(() => window.scrollTo(0, 0));
      await mkdir("../output/playwright", { recursive: true });
      await page.screenshot({ path: `../output/playwright/person-${viewportName}-${theme}.png`, fullPage: true });
      await page.screenshot({ path: `../output/playwright/person-${viewportName}-${theme}-hero.png` });
      await page.getByRole("heading", { name: "Filmography", exact: true }).scrollIntoViewIfNeeded();
      await page.screenshot({ path: `../output/playwright/person-${viewportName}-${theme}-credits.png` });
      assertMockSuiteClean(issues, unexpectedApiRequests);
    });
  }
}
