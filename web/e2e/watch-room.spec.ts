import { expect, test } from "./e2e-test";
import type { Browser } from "@playwright/test";

import {
  intEnv,
  readE2EEnv,
  requireRealInstance,
  type E2EEnv,
} from "./e2e-env";
import {
  MOVIE_SEEK_STEP_SEC,
  WATCH_ROOM_SYNC_DRIFT_THRESHOLD_SEC,
} from "../src/lib/constants";
import { loginViaApi } from "./e2e-auth";
import { deleteUser } from "./e2e-users";
import { realMediaTimeouts } from "./media-e2e-helpers";
import {
  cleanupWatchRoom,
  createGuest,
  createWatchRoom,
  expectLive,
  expectWatchRoomChrome,
  fakeWatchRoomMedia,
  roomPauseButton,
  roomPlayButton,
  videoCurrentTime,
  type E2EGuest,
} from "./watch-room-e2e";

// Watch rooms against a real server: its socket, its membership rules and,
// for the HLS room, its media. The mocked twin is watch-room-mock.spec.ts.

type WatchRoomEnv = E2EEnv & {
  movieId: number | null;
  hlsMovieId: number | null;
  responseTimeoutMs: number;
};

function readWatchRoomEnv(): WatchRoomEnv {
  return {
    ...readE2EEnv(),
    movieId: intEnv("E2E_WATCH_ROOM_MOVIE_ID") || null,
    hlsMovieId: intEnv("E2E_WATCH_ROOM_HLS_MOVIE_ID") || null,
    responseTimeoutMs: intEnv("E2E_WATCH_ROOM_RESPONSE_TIMEOUT_MS", 30_000),
  };
}

const env = readWatchRoomEnv();

// A guest's player starts from another member's command, without a click of
// its own; real media needs the browser to allow that.
test.use({ launchOptions: { args: ["--autoplay-policy=no-user-gesture-required"] } });

// Real media only: this suite's browser noise (media aborts, codec warnings)
// has not been triaged against a real instance yet, so it is not tracked.
test.use({
  browserIssueOptions: { skip: "real-media suite, browser noise not triaged yet" },
});

/** An owner (the configured admin) and invitees, each in their own browser. */
async function openRoomParty(browser: Browser, guestCount: number) {
  const ownerContext = await browser.newContext({ baseURL: env.baseURL });
  await loginViaApi(ownerContext.request, env);
  const guests: E2EGuest[] = [];
  for (let i = 0; i < guestCount; i += 1) {
    guests.push(await createGuest(ownerContext.request, `Watch Room Guest ${i + 1}`));
  }
  const guestContexts = await Promise.all(
    guests.map(async guest => {
      const context = await browser.newContext({ baseURL: env.baseURL });
      await loginViaApi(context.request, guest);
      return context;
    }),
  );

  return {
    ownerContext,
    guests,
    guestContexts,
    async close(roomId: number | null) {
      await cleanupWatchRoom(ownerContext.request, roomId);
      for (const guest of guests) {
        await deleteUser(ownerContext.request, guest.id, { allowMissing: true });
      }
      await Promise.all(guestContexts.map(context => context.close()));
      await ownerContext.close();
    },
  };
}

test.describe("Watch room realtime playback", () => {
  requireRealInstance("watch rooms need a real websocket and media");
  test.describe.configure({ timeout: realMediaTimeouts().test });

  test("syncs direct-room playback controls across owner and guest browsers", async ({
    browser,
  }) => {
    test.skip(!env.movieId, "Set E2E_WATCH_ROOM_MOVIE_ID to run the direct-room test.");
    const party = await openRoomParty(browser, 1);
    let roomId: number | null = null;

    try {
      roomId = await createWatchRoom(party.ownerContext.request, env.movieId!, [
        party.guests[0].id,
      ]);
      const ownerPage = await party.ownerContext.newPage();
      const guestPage = await party.guestContexts[0].newPage();
      await fakeWatchRoomMedia(ownerPage);
      await fakeWatchRoomMedia(guestPage);

      await Promise.all([
        ownerPage.goto(`/watch-rooms/${roomId}`),
        guestPage.goto(`/watch-rooms/${roomId}`),
      ]);
      await expectLive(ownerPage, 2, env.responseTimeoutMs);
      await expectLive(guestPage, 2, env.responseTimeoutMs);
      await expectWatchRoomChrome(ownerPage);
      await expectWatchRoomChrome(guestPage);

      await roomPlayButton(ownerPage).click();
      await expect(
        roomPauseButton(guestPage),
      ).toBeVisible({ timeout: env.responseTimeoutMs });

      await ownerPage
        .getByRole("button", {
          name: `Fast-forward ${MOVIE_SEEK_STEP_SEC} seconds`,
        })
        .click();
      await expect
        .poll(() => videoCurrentTime(guestPage), {
          timeout: env.responseTimeoutMs,
        })
        .toBeGreaterThanOrEqual(MOVIE_SEEK_STEP_SEC - 0.5);

      await roomPauseButton(guestPage).click();
      await expect(
        roomPlayButton(ownerPage),
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
      await party.close(roomId);
    }
  });

  test("edits invitees and lets a guest leave", async ({ browser }) => {
    test.skip(!env.movieId, "Set E2E_WATCH_ROOM_MOVIE_ID to run the members test.");
    const party = await openRoomParty(browser, 2);
    const [removed, added] = party.guests;
    let roomId: number | null = null;

    try {
      roomId = await createWatchRoom(party.ownerContext.request, env.movieId!, [removed.id]);
      const ownerPage = await party.ownerContext.newPage();
      const removedPage = await party.guestContexts[0].newPage();
      const addedPage = await party.guestContexts[1].newPage();
      for (const page of [ownerPage, removedPage, addedPage]) {
        await fakeWatchRoomMedia(page);
      }
      await Promise.all([
        ownerPage.goto(`/watch-rooms/${roomId}`),
        removedPage.goto(`/watch-rooms/${roomId}`),
      ]);
      await expectLive(ownerPage, 2, env.responseTimeoutMs);
      await expectLive(removedPage, 2, env.responseTimeoutMs);

      // The owner swaps the first guest for the second.
      await ownerPage.getByRole("button", { name: /^Edit invitees for/ }).click();
      const dialog = ownerPage.getByRole("dialog", { name: "Edit invitees" });
      await dialog.getByRole("checkbox", { name: `Invite ${removed.name}` }).click();
      await dialog.getByRole("checkbox", { name: `Invite ${added.name}` }).click();
      await dialog.getByRole("button", { name: "Save invitees" }).click();
      await expect(dialog).toBeHidden({ timeout: env.responseTimeoutMs });

      await expect(removedPage).toHaveURL(/\/$/, { timeout: env.responseTimeoutMs });
      await expect(ownerPage.getByText("1 connected now")).toBeVisible({
        timeout: env.responseTimeoutMs,
      });
      await expect(
        ownerPage.getByRole("complementary").getByText(added.name),
      ).toBeVisible();

      // The added guest joins, then leaves for good.
      await addedPage.goto(`/watch-rooms/${roomId}`);
      await expectLive(addedPage, 2, env.responseTimeoutMs);
      await addedPage.getByRole("button", { name: /^Leave watch room for/ }).click();
      await addedPage.getByRole("button", { name: "Leave room" }).click();
      await expect(addedPage).toHaveURL(/\/$/, { timeout: env.responseTimeoutMs });
      await expect(ownerPage.getByText("1 connected now")).toBeVisible({
        timeout: env.responseTimeoutMs,
      });
      await expect(
        ownerPage.getByRole("complementary").getByText(added.name),
      ).toBeHidden();
    } finally {
      await party.close(roomId);
    }
  });
});

test.describe("Watch room HLS playback with real media", () => {
  requireRealInstance("an HLS room needs a real transcode");
  test.describe.configure({ timeout: realMediaTimeouts().test });

  test("keeps two viewers of an HLS room on the same position", async ({ browser }) => {
    test.skip(!env.hlsMovieId, "Set E2E_WATCH_ROOM_HLS_MOVIE_ID to run the HLS-room test.");
    const party = await openRoomParty(browser, 1);
    let roomId: number | null = null;

    try {
      roomId = await createWatchRoom(
        party.ownerContext.request,
        env.hlsMovieId!,
        [party.guests[0].id],
        "720p_3mbps",
      );
      const ownerPage = await party.ownerContext.newPage();
      const guestPage = await party.guestContexts[0].newPage();
      await Promise.all([
        ownerPage.goto(`/watch-rooms/${roomId}`),
        guestPage.goto(`/watch-rooms/${roomId}`),
      ]);
      await expectLive(ownerPage, 2, env.responseTimeoutMs);
      await expectLive(guestPage, 2, env.responseTimeoutMs);

      await roomPlayButton(ownerPage).click();
      await expect
        .poll(() => videoCurrentTime(guestPage), { timeout: env.responseTimeoutMs })
        .toBeGreaterThan(2);

      await ownerPage
        .getByRole("button", { name: `Fast-forward ${MOVIE_SEEK_STEP_SEC} seconds` })
        .click();
      await expect
        .poll(
          async () => {
            const [owner, guest] = await Promise.all([
              videoCurrentTime(ownerPage),
              videoCurrentTime(guestPage),
            ]);
            return owner > MOVIE_SEEK_STEP_SEC ? Math.abs(owner - guest) : Infinity;
          },
          { timeout: env.responseTimeoutMs },
        )
        .toBeLessThan(WATCH_ROOM_SYNC_DRIFT_THRESHOLD_SEC);

      await roomPauseButton(guestPage).click();
      await expect(roomPlayButton(ownerPage)).toBeVisible({
        timeout: env.responseTimeoutMs,
      });
    } finally {
      await party.close(roomId);
    }
  });
});
