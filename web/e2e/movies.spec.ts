import { expect, test, type Page } from "@playwright/test";
import { trackBrowserIssues } from "./e2e-browser-issues";
import { apiURL, readE2EEnv } from "./e2e-env";
import { loginViaApi } from "./e2e-auth";

const desktopViewport = { width: 1440, height: 1200 };
const moviesAllPath =
  "/movies?tab=all&allPage=1&sort=asc&genresPage=1&playlistsPage=1";

async function expectMoviesPageLoaded(page: Page) {
  await expect(page).toHaveTitle("Movies - Igloo");
  await expect(page.getByRole("heading", { name: "Movie Library" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "All Movies" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "Genres" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "Playlists" })).toBeVisible();
  await expect(
    page.getByRole("tabpanel", { name: "All Movies" }),
  ).toBeVisible();
}

async function expectGenresSmoke(page: Page) {
  await page.getByRole("tab", { name: "Genres" }).click();
  await expect(page.getByRole("tab", { name: "Genres" })).toHaveAttribute(
    "aria-selected",
    "true",
  );

  const genresPanel = page.getByRole("tabpanel", { name: "Genres" });
  await expect(genresPanel).toBeVisible();

  const emptyState = genresPanel.getByText(
    "No genres with movies in your library yet.",
  );
  const genresList = genresPanel.getByRole("list", { name: "Movie genres" });

  await expect
    .poll(async () => {
      if (await emptyState.isVisible()) {
        return "empty";
      }

      if (await genresList.isVisible()) {
        return "populated";
      }

      return "loading";
    })
    .not.toBe("loading");

  if (await emptyState.isVisible()) {
    await expect(emptyState).toBeVisible();
    return;
  }

  await expect(genresList).toBeVisible();
  await expect(genresList.getByRole("button").first()).toBeVisible();
}

async function expectPlaylistsSmoke(page: Page) {
  await page.getByRole("tab", { name: "Playlists" }).click();
  await expect(page.getByRole("tab", { name: "Playlists" })).toHaveAttribute(
    "aria-selected",
    "true",
  );

  const playlistsPanel = page.getByRole("tabpanel", { name: "Playlists" });
  await expect(playlistsPanel).toBeVisible();

  const likedMoviesButton = playlistsPanel.getByRole("button", {
    name: "Liked movies",
  });
  const newPlaylistButton = playlistsPanel.getByRole("button", {
    name: "New playlist",
  });

  await expect(likedMoviesButton).toBeVisible();
  await expect(newPlaylistButton).toBeVisible();

  const emptyPlaylistsHeading = playlistsPanel.getByRole("heading", {
    name: "No movie playlists yet",
  });
  const firstPlaylistLink = playlistsPanel
    .locator('a[href^="/movies/playlist/"]')
    .first();

  await expect
    .poll(async () => {
      if (await emptyPlaylistsHeading.isVisible()) {
        return "empty";
      }

      if (await firstPlaylistLink.isVisible()) {
        return "populated";
      }

      return "loading";
    })
    .not.toBe("loading");

  if (await emptyPlaylistsHeading.isVisible()) {
    await expect(emptyPlaylistsHeading).toBeVisible();
  } else {
    await expect(firstPlaylistLink).toBeVisible();
  }

  await likedMoviesButton.click();

  await expect(page.getByRole("tab", { name: "Playlists" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await expect(page).toHaveURL(/tab=playlists/);
  await expect(page).toHaveURL(/view=liked/);

  const backToPlaylistsButton = page.getByRole("button", {
    name: "Back to playlists",
  });
  await expect(backToPlaylistsButton).toBeVisible();

  const likedEmptyState = playlistsPanel.getByText(
    "You have not liked any movies yet.",
  );
  const firstLikedMovieLink = playlistsPanel
    .locator('a[href^="/movies/"]')
    .first();

  await expect
    .poll(async () => {
      if (await likedEmptyState.isVisible()) {
        return "empty";
      }

      if (await firstLikedMovieLink.isVisible()) {
        return "populated";
      }

      return "loading";
    })
    .not.toBe("loading");

  if (await likedEmptyState.isVisible()) {
    await expect(likedEmptyState).toBeVisible();
  } else {
    await expect(firstLikedMovieLink).toBeVisible();
  }

  await backToPlaylistsButton.click();

  await expect(page).not.toHaveURL(/view=liked/);
  await expect(page.getByRole("tab", { name: "Playlists" })).toHaveAttribute(
    "aria-selected",
    "true",
  );
  await expect(page.getByRole("button", { name: "Liked movies" })).toBeVisible();
  await expect(page.getByRole("button", { name: "New playlist" })).toBeVisible();
}

test.describe("movies page", () => {
  test("loads live tabs and handles empty or populated genres and playlists", async ({
    page,
  }) => {
    const env = readE2EEnv();
    await loginViaApi(page.context().request, env);

    const browserIssues = trackBrowserIssues(page);
    await page.setViewportSize(desktopViewport);
    await page.goto(apiURL(env, moviesAllPath));

    await expectMoviesPageLoaded(page);
    await expectGenresSmoke(page);
    await expectPlaylistsSmoke(page);

    browserIssues.assertClean();
  });
});
