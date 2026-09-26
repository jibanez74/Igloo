import { expect, test, type APIRequestContext, type Page } from "@playwright/test";

import {
  intEnv,
  readE2EEnv,
  type Credentials,
  type E2EEnv,
} from "./e2e-env";
import { expectPageHasNoHorizontalScroll } from "./e2e-layout";
import { WATCH_ROOM_SEEK_STEP_SEC } from "../src/lib/constants";
import { readJSON } from "./e2e-api";
import { loginViaApi } from "./e2e-auth";
import { createUser, deleteUser } from "./e2e-users";

type WatchRoomEnv = E2EEnv & {
  movieId: number;
  responseTimeoutMs: number;
};

type E2EGuest = Credentials & {
  id: number;
};

function readWatchRoomEnv(): WatchRoomEnv | null {
  const movieId = intEnv("E2E_WATCH_ROOM_MOVIE_ID");

  if (!movieId) {
    return null;
  }

  return {
    ...readE2EEnv(),
    movieId,
    responseTimeoutMs: intEnv("E2E_WATCH_ROOM_RESPONSE_TIMEOUT_MS", 30_000),
  };
}

async function createGuest(request: APIRequestContext): Promise<E2EGuest> {
  const unique = Date.now().toString(36);
  const credentials = {
    email: `watch-room-e2e-${unique}@example.test`,
    password: `WatchRoom-${unique}-pass`,
  };
  const user = await createUser(request, {
    name: "Watch Room E2E Guest",
    ...credentials,
  });

  return { id: user.id, ...credentials };
}

async function createWatchRoom(
  request: APIRequestContext,
  movieId: number,
  guestId: number,
) {
  const response = await request.post("/api/watch-rooms", {
    data: {
      movie_id: movieId,
      mode: "direct",
      audio_track: 0,
      subtitle_track: null,
      invited_user_ids: [guestId],
    },
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(201);

  const body = await readJSON<{ room_id: number }>(response);
  expect(body.error, body.message).toBe(false);
  return body.data!.room_id;
}

async function cleanupWatchRoom(request: APIRequestContext, roomId: number | null) {
  if (!roomId) return;

  const response = await request.delete(`/api/watch-rooms/${roomId}`, {
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

async function expectWatchRoomChrome(page: Page) {
  await expect(
    page.getByRole("button", {
      name: `Rewind ${WATCH_ROOM_SEEK_STEP_SEC} seconds`,
    }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "Play playback" })).toBeVisible();
  await expect(
    page.getByRole("button", {
      name: `Fast-forward ${WATCH_ROOM_SEEK_STEP_SEC} seconds`,
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Adjust volume" }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "Fullscreen" })).toBeVisible();
  await expectPageHasNoHorizontalScroll(page);
}

const watchRoomEnv = readWatchRoomEnv();

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
      await loginViaApi(ownerContext.request, env);
      guest = await createGuest(ownerContext.request);
      roomId = await createWatchRoom(
        ownerContext.request,
        env.movieId,
        guest.id,
      );
      await loginViaApi(guestContext.request, guest);

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
      await expectWatchRoomChrome(ownerPage);
      await expectWatchRoomChrome(guestPage);

      await ownerPage.getByRole("button", { name: "Play playback" }).click();
      await expect(
        guestPage.getByRole("button", { name: "Pause playback" }),
      ).toBeVisible({ timeout: env.responseTimeoutMs });

      await ownerPage
        .getByRole("button", {
          name: `Fast-forward ${WATCH_ROOM_SEEK_STEP_SEC} seconds`,
        })
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
      if (guest) await deleteUser(ownerContext.request, guest.id);
      await guestContext.close();
      await ownerContext.close();
    }
  });
});
