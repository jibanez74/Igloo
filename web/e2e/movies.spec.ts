import {
  expect,
  test,
  type APIRequestContext,
  type APIResponse,
  type Page,
} from "@playwright/test";
import { trackBrowserIssues } from "./e2e-browser-issues";
import { apiURL as appURL, readE2EEnv, type E2EEnv } from "./e2e-env";

type ApiResponse<T> = {
  error: boolean;
  message?: string;
  data?: T;
};

const desktopViewport = { width: 1440, height: 1200 };
const moviesAllPath =
  "/movies?tab=all&allPage=1&sort=asc&genresPage=1&playlistsPage=1";

async function readJSON<T>(response: APIResponse) {
  return (await response.json()) as ApiResponse<T>;
}

async function login(request: APIRequestContext, env: E2EEnv) {
  const response = await request.post(appURL(env, "/api/auth/login"), {
    data: { email: env.email, password: env.password },
    failOnStatusCode: false,
  });

  expect(response.status()).toBe(200);

  const body = await readJSON<unknown>(response);
  expect(body.error, body.message).toBe(false);
}

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

  if (await emptyState.isVisible()) {
    await expect(emptyState).toBeVisible();
    return;
  }

  const genresList = genresPanel.getByRole("list", { name: "Movie genres" });
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

  if (await emptyPlaylistsHeading.isVisible()) {
    await expect(emptyPlaylistsHeading).toBeVisible();
  } else {
    await expect(
      playlistsPanel.locator('a[href^="/movies/playlist/"]').first(),
    ).toBeVisible();
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

  if (await likedEmptyState.isVisible()) {
    await expect(likedEmptyState).toBeVisible();
  } else {
    await expect(playlistsPanel.locator('a[href^="/movies/"]').first()).toBeVisible();
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
    await login(page.context().request, env);

    const browserIssues = trackBrowserIssues(page);
    await page.setViewportSize(desktopViewport);
    await page.goto(appURL(env, moviesAllPath));

    await expectMoviesPageLoaded(page);
    await expectGenresSmoke(page);
    await expectPlaylistsSmoke(page);

    browserIssues.assertClean();
  });
});
