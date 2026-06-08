import {
  expect,
  test,
  type APIRequestContext,
  type APIResponse,
  type Page,
} from "@playwright/test";

import { readE2EEnv, type E2EEnv } from "./e2e-env";

type ApiResponse<T> = {
  error: boolean;
  message?: string;
  data?: T;
};

type AdminUser = {
  id: number;
  name: string;
  email: string;
  is_admin: boolean;
};

type WatchRoomEnv = E2EEnv & {
  movieId: number;
  responseTimeoutMs: number;
};

type E2EGuest = {
  id: number;
  email: string;
  password: string;
};

function positiveIntEnv(name: string, fallback?: number) {
  const raw = process.env[name];
  if (!raw) return fallback;

  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback;
}

function readWatchRoomEnv(): WatchRoomEnv | null {
  const e2eEnv = readE2EEnv();
  const movieId = positiveIntEnv("E2E_WATCH_ROOM_MOVIE_ID");

  if (!movieId) {
    return null;
  }

  return {
    ...e2eEnv,
    movieId,
    responseTimeoutMs:
      positiveIntEnv("E2E_WATCH_ROOM_RESPONSE_TIMEOUT_MS", 30_000) ?? 30_000,
  };
}

async function expectApiOk(
  responsePromise: Promise<APIResponse>,
  expectedStatus: number,
) {
  const response = await responsePromise;
  expect(response.status()).toBe(expectedStatus);

  const body = (await response.json()) as ApiResponse<unknown>;
  expect(body.error, body.message).toBe(false);
  return body;
}

async function expectApiData<T>(
  responsePromise: Promise<APIResponse>,
  expectedStatus: number,
) {
  const body = (await expectApiOk(responsePromise, expectedStatus)) as ApiResponse<T>;
  expect(body.data).toBeTruthy();
  return body.data!;
}

async function login(
  request: APIRequestContext,
  email: string,
  password: string,
) {
  await expectApiOk(
    request.post("/api/auth/login", {
      data: {
        email,
        password,
      },
      failOnStatusCode: false,
    }),
    200,
  );

  await expectApiOk(
    request.get("/api/auth/user", {
      failOnStatusCode: false,
    }),
    200,
  );
}

async function createGuest(request: APIRequestContext): Promise<E2EGuest> {
  const unique = Date.now().toString(36);
  const password = `WatchRoom-${unique}-pass`;
  const email = `watch-room-e2e-${unique}@example.test`;

  const data = await expectApiData<{ user: AdminUser }>(
    request.post("/api/admin/users", {
      data: {
        name: "Watch Room E2E Guest",
        email,
        password,
        is_admin: false,
      },
      failOnStatusCode: false,
    }),
    201,
  );

  return {
    id: data.user.id,
    email,
    password,
  };
}

async function createWatchRoom(
  request: APIRequestContext,
  movieId: number,
  guestId: number,
) {
  const data = await expectApiData<{ room_id: number }>(
    request.post("/api/watch-rooms", {
      data: {
        movie_id: movieId,
        mode: "direct",
        audio_track: 0,
        subtitle_track: null,
        invited_user_ids: [guestId],
      },
      failOnStatusCode: false,
    }),
    201,
  );

  return data.room_id;
}

async function cleanupWatchRoom(request: APIRequestContext, roomId: number | null) {
  if (!roomId) return;

  const response = await request.delete(`/api/watch-rooms/${roomId}`, {
    failOnStatusCode: false,
  });
  expect([200, 404]).toContain(response.status());
}

async function cleanupGuest(request: APIRequestContext, guest: E2EGuest | null) {
  if (!guest) return;

  const response = await request.delete(`/api/admin/users/${guest.id}`, {
    failOnStatusCode: false,
  });
  expect([200, 404]).toContain(response.status());
}

async function prepareWatchRoomPage(page: Page) {
  await page.addInitScript(() => {
    type MediaState = {
      currentTime: number;
      duration: number;
      paused: boolean;
      src: string;
    };

    const states = new WeakMap<HTMLMediaElement, MediaState>();
    const stateFor = (media: HTMLMediaElement) => {
      let state = states.get(media);
      if (!state) {
        state = {
          currentTime: 0,
          duration: 120,
          paused: true,
          src: "",
        };
        states.set(media, state);
      }
      return state;
    };

    const dispatchMediaReady = (media: HTMLMediaElement) => {
      queueMicrotask(() => {
        media.dispatchEvent(new Event("durationchange"));
        media.dispatchEvent(new Event("loadedmetadata"));
        media.dispatchEvent(new Event("canplay"));
      });
    };

    Object.defineProperty(HTMLMediaElement.prototype, "readyState", {
      configurable: true,
      get() {
        return 4;
      },
    });
    Object.defineProperty(HTMLMediaElement.prototype, "duration", {
      configurable: true,
      get() {
        return stateFor(this).duration;
      },
    });
    Object.defineProperty(HTMLMediaElement.prototype, "currentTime", {
      configurable: true,
      get() {
        return stateFor(this).currentTime;
      },
      set(value: number) {
        const state = stateFor(this);
        state.currentTime = Math.max(0, Number(value) || 0);
        this.dispatchEvent(new Event("timeupdate"));
      },
    });
    Object.defineProperty(HTMLMediaElement.prototype, "paused", {
      configurable: true,
      get() {
        return stateFor(this).paused;
      },
    });
    Object.defineProperty(HTMLMediaElement.prototype, "src", {
      configurable: true,
      get() {
        return stateFor(this).src;
      },
      set(value: string) {
        stateFor(this).src = String(value);
        dispatchMediaReady(this);
      },
    });

    HTMLMediaElement.prototype.play = async function play() {
      const state = stateFor(this);
      state.paused = false;
      this.dispatchEvent(new Event("play"));
    };
    HTMLMediaElement.prototype.pause = function pause() {
      const state = stateFor(this);
      state.paused = true;
      this.dispatchEvent(new Event("pause"));
    };
    HTMLMediaElement.prototype.load = function load() {
      dispatchMediaReady(this);
    };
  });

  await page.route("**/api/watch-rooms/*/stream", async route => {
    await route.fulfill({
      status: 200,
      contentType: "video/mp4",
      body: "",
    });
  });
}

async function videoCurrentTime(page: Page) {
  return page.locator("video").evaluate(video => {
    return video instanceof HTMLVideoElement ? video.currentTime : 0;
  });
}

const watchRoomEnv = readWatchRoomEnv();

test.describe.configure({ mode: "serial" });

test.describe("Watch room realtime playback", () => {
  test.skip(
    !watchRoomEnv,
    "Set E2E_WATCH_ROOM_MOVIE_ID to run watch-room e2e tests.",
  );

  test("syncs direct-room playback controls across owner and guest browsers", async ({
    browser,
  }) => {
    const env = watchRoomEnv!;
    const ownerContext = await browser.newContext({ baseURL: env.baseURL });
    const guestContext = await browser.newContext({ baseURL: env.baseURL });
    let guest: E2EGuest | null = null;
    let roomId: number | null = null;

    try {
      await login(ownerContext.request, env.email, env.password);
      guest = await createGuest(ownerContext.request);
      roomId = await createWatchRoom(
        ownerContext.request,
        env.movieId,
        guest.id,
      );
      await login(guestContext.request, guest.email, guest.password);

      const ownerPage = await ownerContext.newPage();
      const guestPage = await guestContext.newPage();
      await prepareWatchRoomPage(ownerPage);
      await prepareWatchRoomPage(guestPage);

      await Promise.all([
        ownerPage.goto(`/watch-rooms/${roomId}`),
        guestPage.goto(`/watch-rooms/${roomId}`),
      ]);

      await expect(ownerPage.getByText("Realtime sync connected")).toBeVisible({
        timeout: env.responseTimeoutMs,
      });
      await expect(guestPage.getByText("Realtime sync connected")).toBeVisible({
        timeout: env.responseTimeoutMs,
      });
      await expect(ownerPage.getByText("2 connected now")).toBeVisible({
        timeout: env.responseTimeoutMs,
      });
      await expect(guestPage.getByText("2 connected now")).toBeVisible({
        timeout: env.responseTimeoutMs,
      });

      await ownerPage.getByRole("button", { name: "Play playback" }).click();
      await expect(
        guestPage.getByRole("button", { name: "Pause playback" }),
      ).toBeVisible({ timeout: env.responseTimeoutMs });

      await ownerPage
        .getByRole("button", { name: "Fast-forward 10 seconds" })
        .click();
      await expect
        .poll(() => videoCurrentTime(guestPage), {
          timeout: env.responseTimeoutMs,
        })
        .toBeGreaterThanOrEqual(9.5);

      await guestPage.getByRole("button", { name: "Pause playback" }).click();
      await expect(
        ownerPage.getByRole("button", { name: "Play playback" }),
      ).toBeVisible({ timeout: env.responseTimeoutMs });

      await ownerPage
        .getByRole("button", { name: /close watch room for/i })
        .click();
      await ownerPage.getByRole("button", { name: "Close room" }).click();

      await expect(guestPage).toHaveURL(/\/$/, {
        timeout: env.responseTimeoutMs,
      });
      roomId = null;
    } finally {
      await cleanupWatchRoom(ownerContext.request, roomId);
      await cleanupGuest(ownerContext.request, guest);
      await guestContext.close();
      await ownerContext.close();
    }
  });
});
