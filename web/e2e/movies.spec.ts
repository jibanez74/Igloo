import {
  expect,
  test,
  type APIRequestContext,
  type APIResponse,
  type Page,
  type Request,
  type Response,
} from "@playwright/test";

type ApiResponse<T> = {
  error: boolean;
  message?: string;
  data?: T;
};

type MoviesEnv = {
  baseURL: string;
  email: string;
  password: string;
};

type Region = {
  x: number;
  y: number;
  width: number;
  height: number;
};

type TransitionFrames = {
  region: Region;
  immediate: string;
  after100: string;
  after250: string;
};

const desktopViewport = { width: 1440, height: 1200 };
const moviesAllPath =
  "/movies?tab=all&allPage=1&sort=asc&genresPage=1&playlistsPage=1";
const moviesGenresPath =
  "/movies?tab=genres&allPage=1&sort=asc&genresPage=1&playlistsPage=1";
const visibleMotionThreshold = 4; // Mean per-channel RGB delta across sampled tab-content frames; a heuristic floor where the fade reads as visibly moving instead of normal capture noise.
const reducedMotionDriftThreshold = 2; // Mean per-channel RGB drift allowed with `prefers-reduced-motion`; a heuristic tolerance for screenshot/layout jitter while still treating the transition as effectively static.

function readMoviesEnv(): MoviesEnv {
  return {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3000",
    email: process.env.E2E_ADMIN_EMAIL ?? "admin@example.com",
    password: process.env.E2E_ADMIN_PASSWORD ?? "AdminPassword",
  };
}

function appURL(env: MoviesEnv, path: string) {
  return new URL(path, env.baseURL).toString();
}

function isAppApiResponse(response: Response) {
  return new URL(response.url()).pathname.startsWith("/api/");
}

function isIgnorableFailedRequest(request: Request) {
  const failureText = request.failure()?.errorText ?? "";
  const resourceType = request.resourceType();

  return (
    failureText.includes("net::ERR_ABORTED") &&
    (resourceType === "image" || resourceType === "font")
  );
}

async function readJSON<T>(response: APIResponse) {
  return (await response.json()) as ApiResponse<T>;
}

async function login(request: APIRequestContext, env: MoviesEnv) {
  const response = await request.post(appURL(env, "/api/auth/login"), {
    data: { email: env.email, password: env.password },
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(200);

  const body = await readJSON<unknown>(response);
  expect(body.error, body.message).toBe(false);
}

function trackBrowserIssues(page: Page) {
  const consoleIssues: string[] = [];
  const pageErrors: string[] = [];
  const failedRequests: string[] = [];
  const responseErrors: string[] = [];

  page.on("console", message => {
    if (message.type() === "error" || message.type() === "warning") {
      consoleIssues.push(`${message.type()}: ${message.text()}`);
    }
  });
  page.on("pageerror", error => pageErrors.push(error.message));
  page.on("requestfailed", request => {
    if (isIgnorableFailedRequest(request)) {
      return;
    }

    failedRequests.push(
      `${request.method()} ${request.url()} ${request.failure()?.errorText ?? ""}`,
    );
  });
  page.on("response", response => {
    if (isAppApiResponse(response) && response.status() >= 400) {
      responseErrors.push(
        `${response.status()} ${response.request().method()} ${response.url()}`,
      );
    }
  });

  return {
    assertClean() {
      expect(consoleIssues).toEqual([]);
      expect(pageErrors).toEqual([]);
      expect(failedRequests).toEqual([]);
      expect(responseErrors).toEqual([]);
    },
  };
}

async function screenshotDataURL(page: Page) {
  const buffer = await page.screenshot({
    fullPage: false,
    scale: "css",
    type: "png",
  });

  return `data:image/png;base64,${buffer.toString("base64")}`;
}

async function contentRegion(page: Page) {
  return page.evaluate(() => {
    const tablist = document.querySelector('[role="tablist"]');
    if (!tablist) {
      throw new Error("Could not find tablist for transition region.");
    }

    const rect = tablist.getBoundingClientRect();
    const y = Math.min(window.innerHeight - 1, rect.bottom + 12);

    return {
      x: Math.max(0, rect.left),
      y,
      width: Math.max(1, window.innerWidth - rect.left - 24),
      height: Math.max(1, window.innerHeight - y - 24),
    };
  });
}

async function meanRGBDiff(page: Page, first: string, second: string, region: Region) {
  return page.evaluate(
    async ({ first, second, region }) => {
      const loadImage = (source: string) =>
        new Promise<HTMLImageElement>((resolve, reject) => {
          const image = new Image();
          image.onload = () => resolve(image);
          image.onerror = () => reject(new Error("Could not decode screenshot."));
          image.src = source;
        });

      const [firstImage, secondImage] = await Promise.all([
        loadImage(first),
        loadImage(second),
      ]);
      const canvas = document.createElement("canvas");
      canvas.width = firstImage.width;
      canvas.height = firstImage.height;

      const context = canvas.getContext("2d", { willReadFrequently: true });
      if (!context) {
        throw new Error("Could not create image diff canvas context.");
      }

      const x = Math.max(0, Math.floor(region.x));
      const y = Math.max(0, Math.floor(region.y));
      const width = Math.max(
        1,
        Math.min(
          Math.floor(region.width),
          firstImage.width - x,
          secondImage.width - x,
        ),
      );
      const height = Math.max(
        1,
        Math.min(
          Math.floor(region.height),
          firstImage.height - y,
          secondImage.height - y,
        ),
      );

      context.drawImage(firstImage, 0, 0);
      const firstData = context.getImageData(x, y, width, height).data;

      context.clearRect(0, 0, canvas.width, canvas.height);
      context.drawImage(secondImage, 0, 0);
      const secondData = context.getImageData(x, y, width, height).data;

      let totalDiff = 0;
      for (let index = 0; index < firstData.length; index += 4) {
        totalDiff += Math.abs(firstData[index] - secondData[index]);
        totalDiff += Math.abs(firstData[index + 1] - secondData[index + 1]);
        totalDiff += Math.abs(firstData[index + 2] - secondData[index + 2]);
      }

      return totalDiff / (width * height * 3);
    },
    { first, second, region },
  );
}

async function maxContentMotion(page: Page, frames: TransitionFrames) {
  const diffs = [
    await meanRGBDiff(page, frames.immediate, frames.after100, frames.region),
    await meanRGBDiff(page, frames.after100, frames.after250, frames.region),
    await meanRGBDiff(page, frames.immediate, frames.after250, frames.region),
  ];

  return Math.max(...diffs);
}

async function captureTabTransition(page: Page, tabName: string) {
  const region = await contentRegion(page);

  await page.getByRole("tab", { name: tabName }).click();
  const immediate = await screenshotDataURL(page);
  await page.waitForTimeout(100);
  const after100 = await screenshotDataURL(page);
  await page.waitForTimeout(150);
  const after250 = await screenshotDataURL(page);

  return {
    region,
    immediate,
    after100,
    after250,
  };
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
  await expect(page.getByText(/Page 1 of \d+/)).toBeVisible();
}

test.describe("movies page", () => {
  test("loads, switches tabs, and opens liked movies from the overflow menu", async ({
    page,
  }) => {
    const env = readMoviesEnv();
    await login(page.context().request, env);

    const browserIssues = trackBrowserIssues(page);
    await page.setViewportSize(desktopViewport);
    await page.goto(appURL(env, moviesAllPath));

    await expectMoviesPageLoaded(page);

    await page.getByRole("tab", { name: "Genres" }).click();
    await expect(page.getByRole("tabpanel", { name: "Genres" })).toBeVisible();
    await expect(page.getByRole("button", { name: /Action \d+ movies/ })).toBeVisible();

    await page.getByRole("tab", { name: "Playlists" }).click();
    await expect(
      page.getByRole("tabpanel", { name: "Playlists" }),
    ).toBeVisible();
    await expect(page.getByRole("button", { name: "Liked movies" })).toBeVisible();
    await expect(page.getByRole("button", { name: "New playlist" })).toBeVisible();

    await page.getByRole("tab", { name: "All Movies" }).click();
    await expect(
      page.getByRole("tabpanel", { name: "All Movies" }),
    ).toBeVisible();

    await page.getByRole("button", { name: "More options" }).click();
    await page.getByRole("menuitem", { name: "Liked movies" }).click();

    await expect(page.getByRole("tab", { name: "Playlists" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await expect(page).toHaveURL(/tab=playlists/);
    await expect(page).toHaveURL(/view=liked/);
    await expect(page.getByRole("button", { name: "Back to playlists" })).toBeVisible();

    const activeElementName = await page.evaluate(() => {
      const active = document.activeElement;
      if (!(active instanceof HTMLElement)) {
        return "";
      }

      return active.getAttribute("aria-label") ?? active.textContent?.trim() ?? "";
    });
    expect(activeElementName.length).toBeGreaterThan(0);

    await page.getByRole("button", { name: "Back to playlists" }).click();
    await expect(page).not.toHaveURL(/view=liked/);
    await expect(page.getByRole("button", { name: "Liked movies" })).toBeVisible();
    await expect(page.getByRole("button", { name: "New playlist" })).toBeVisible();

    browserIssues.assertClean();
  });

  test("matches settings tab fade behavior and respects reduced motion", async ({
    page,
  }) => {
    const env = readMoviesEnv();
    await login(page.context().request, env);

    const browserIssues = trackBrowserIssues(page);
    await page.setViewportSize(desktopViewport);

    await page.goto(appURL(env, "/settings"));
    await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
    await expect(page.getByRole("tabpanel", { name: "General" })).toBeVisible();

    const settingsFrames = await captureTabTransition(page, "Account");
    await expect(page.getByRole("tabpanel", { name: "Account" })).toBeVisible();
    const settingsMotion = await maxContentMotion(page, settingsFrames);

    expect(settingsMotion).toBeGreaterThan(visibleMotionThreshold);

    await page.goto(appURL(env, moviesAllPath));
    await expectMoviesPageLoaded(page);

    const moviesFrames = await captureTabTransition(page, "Genres");
    await expect(page.getByRole("tabpanel", { name: "Genres" })).toBeVisible();
    const moviesMotion = await maxContentMotion(page, moviesFrames);

    expect(moviesMotion).toBeGreaterThan(visibleMotionThreshold);
    expect(moviesMotion).toBeGreaterThanOrEqual(settingsMotion * 0.2);

    await page.emulateMedia({ reducedMotion: "reduce" });
    await page.goto(appURL(env, moviesAllPath));
    await expectMoviesPageLoaded(page);
    await expect
      .poll(async () =>
        page.evaluate(() =>
          window.matchMedia("(prefers-reduced-motion: reduce)").matches,
        ),
      )
      .toBe(true);

    const reducedFrames = await captureTabTransition(page, "Genres");
    await expect(page.getByRole("tabpanel", { name: "Genres" })).toBeVisible();
    const reducedMotion = await maxContentMotion(page, {
      ...reducedFrames,
      after250: reducedFrames.after100,
    });

    expect(reducedMotion).toBeLessThan(reducedMotionDriftThreshold);

    await page.emulateMedia({ reducedMotion: "no-preference" });
    await page.goto(appURL(env, moviesGenresPath));
    await expect(page.getByRole("tabpanel", { name: "Genres" })).toBeVisible();
    const playlistsFrames = await captureTabTransition(page, "Playlists");
    await expect(
      page.getByRole("tabpanel", { name: "Playlists" }),
    ).toBeVisible();
    expect(await maxContentMotion(page, playlistsFrames)).toBeGreaterThan(
      visibleMotionThreshold,
    );

    const allMoviesFrames = await captureTabTransition(page, "All Movies");
    await expect(
      page.getByRole("tabpanel", { name: "All Movies" }),
    ).toBeVisible();
    expect(await maxContentMotion(page, allMoviesFrames)).toBeGreaterThan(
      visibleMotionThreshold,
    );

    browserIssues.assertClean();
  });
});
