import { expect, type APIRequestContext, type Page } from "@playwright/test";

import { WATCH_ROOM_SEEK_STEP_SEC } from "../src/lib/constants";
import { readJSON } from "./e2e-api";
import type { Credentials } from "./e2e-env";
import { expectPageHasNoHorizontalScroll } from "./e2e-layout";
import { createUser } from "./e2e-users";

// Shared by the mocked and the real-instance watch-room specs.

export type E2EGuest = Credentials & {
  id: number;
  name: string;
};

let guestCount = 0;

/** A disposable invitee; `request` holds an admin session. */
export async function createGuest(
  request: APIRequestContext,
  name = "Watch Room E2E Guest",
): Promise<E2EGuest> {
  guestCount += 1;
  const unique = `${Date.now().toString(36)}-${guestCount}`;
  const credentials = {
    email: `watch-room-e2e-${unique}@example.test`,
    password: `WatchRoom-${unique}-pass`,
  };
  const user = await createUser(request, { name, ...credentials });

  return { id: user.id, name, ...credentials };
}

export async function createWatchRoom(
  request: APIRequestContext,
  movieId: number,
  guestIds: number[],
  mode = "direct",
) {
  const response = await request.post("/api/watch-rooms", {
    data: {
      movie_id: movieId,
      mode,
      audio_track: 0,
      subtitle_track: null,
      invited_user_ids: guestIds,
    },
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(201);

  const body = await readJSON<{ room_id: number }>(response);
  expect(body.error, body.message).toBe(false);
  return body.data!.room_id;
}

export async function cleanupWatchRoom(request: APIRequestContext, roomId: number | null) {
  if (!roomId) return;

  const response = await request.delete(`/api/watch-rooms/${roomId}`, {
    failOnStatusCode: false,
  });
  expect([200, 404]).toContain(response.status());
}

/**
 * Replaces the page's media elements with a fake that is always ready and
 * keeps its own clock, and answers the room's direct stream with an empty
 * MP4: the sync is what the specs check, not decoding.
 */
export async function fakeWatchRoomMedia(page: Page) {
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


export async function videoCurrentTime(page: Page) {
  return page.locator("video").evaluate(video => {
    return video instanceof HTMLVideoElement ? video.currentTime : 0;
  });
}

export async function expectWatchRoomChrome(page: Page) {
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
