import { expect, test, type BrowserIssues, type TrackedPage } from "./e2e-test";
import {
  isApiResourceEcho,
  isSessionEndedResponse,
  type TrackBrowserIssuesOptions,
} from "./e2e-browser-issues";
import type { Browser, BrowserContext, Page } from "@playwright/test";

import { MOVIE_SEEK_STEP_SEC, WATCH_ROOM_CONNECT_TIMEOUT_MS } from "../src/lib/constants";
import { gateRoute } from "./e2e-api";
import { loginViaApi, submitLogin } from "./e2e-auth";
import { readE2EEnv, requireMockApi, type Credentials } from "./e2e-env";
import { MOCK_MOVIE_ID } from "./fixtures/movies";
import {
  createGuest,
  createWatchRoom,
  expectLive,
  fakeWatchRoomMedia,
  roomPauseButton,
  roomPlayButton,
  videoCurrentTime,
  type E2EGuest,
} from "./watch-room-e2e";

// Watch rooms against the mock API's rooms and socket (mock-watch-rooms.ts):
// every flow a member can take, two or three browsers at a time. The real
// server runs the same flows in watch-room.spec.ts.

requireMockApi();

const MOVIE_TITLE = "Signal Fire";
const admin = readE2EEnv();
/** Home's rooms list (GET), not a single room's routes under it. */
const ROOMS_LIST = /\/api\/watch-rooms$/;

type Viewer = { context: BrowserContext; page: Page };

async function openViewer(
  browser: Browser,
  credentials: Credentials = admin,
): Promise<Viewer> {
  const context = await browser.newContext();
  await loginViaApi(context.request, credentials);
  const page = await context.newPage();
  await fakeWatchRoomMedia(page);
  return { context, page };
}

/** The 403 a viewer outside the room hears, and its console echo. */
function roomRefused(roomId: number): TrackBrowserIssuesOptions {
  return {
    ignoreResponse: response =>
      response.status() === 403 && response.url().includes(`/api/watch-rooms/${roomId}`),
    ignoreConsole: (_type, text) => text.includes("status of 403"),
  };
}

test.describe("Watch rooms", () => {
  let owner: Viewer;
  let ownerIssues: TrackedPage;
  const others: Viewer[] = [];
  // The running test's tracker: every viewer's page is watched too (the
  // default page stays blank in these tests).
  let tracked: BrowserIssues;

  test.beforeEach(async ({ browser, browserIssues }) => {
    tracked = browserIssues;
    owner = await openViewer(browser);
    ownerIssues = tracked.track(owner.page);
  });

  test.afterEach(async () => {
    for (const viewer of [owner, ...others.splice(0)]) {
      await viewer.context.close();
    }
  });

  async function guestViewer(
    browser: Browser,
    guest: E2EGuest,
    issueOptions?: TrackBrowserIssuesOptions,
  ) {
    const viewer = await openViewer(browser, guest);
    const issues = tracked.track(viewer.page, issueOptions);
    others.push(viewer);
    return { ...viewer, issues };
  }

  test("an owner opens a room from the movie, a guest joins from Home, and both share playback", async ({
    browser,
  }) => {
    const guest = await createGuest(owner.context.request, "Couch Guest");

    await owner.page.goto(`/movies/${MOCK_MOVIE_ID}`);
    await owner.page.getByRole("button", { name: "More options" }).click();
    await owner.page.getByRole("menuitem", { name: "Watch Together" }).click();
    const dialog = owner.page.getByRole("dialog", { name: "Watch together" });
    await dialog.getByRole("checkbox", { name: `Invite ${guest.name}` }).click();
    await dialog.getByRole("button", { name: "Create and join room" }).click();
    await expect(owner.page).toHaveURL(/\/watch-rooms\/\d+$/);
    await expectLive(owner.page, 1);

    const { page: guestPage, issues: guestIssues } = await guestViewer(browser, guest);
    await guestPage.goto("/");
    const card = guestPage.getByRole("article", { name: `Watch room: ${MOVIE_TITLE}` });
    await card.getByRole("link", { name: `Join watch room for ${MOVIE_TITLE}` }).click();
    await expectLive(guestPage, 2);
    await expectLive(owner.page, 2);

    // Play and seek from the owner, pause from the guest.
    await roomPlayButton(owner.page).click();
    await expect(roomPauseButton(guestPage)).toBeVisible();
    await owner.page
      .getByRole("button", { name: `Fast-forward ${MOVIE_SEEK_STEP_SEC} seconds` })
      .click();
    await expect
      .poll(() => videoCurrentTime(guestPage))
      .toBeGreaterThanOrEqual(MOVIE_SEEK_STEP_SEC - 0.5);
    await roomPauseButton(guestPage).click();
    await expect(roomPlayButton(owner.page)).toBeVisible();
    const pausedAt = await videoCurrentTime(owner.page);

    // A network blip: both pages say so, come back, and keep the room's place.
    const roomId = Number(owner.page.url().split("/").at(-1));
    const drop = await owner.context.request.post(`/api/e2e/watch-rooms/${roomId}/drop`);
    expect(drop.ok()).toBe(true);
    // Both pages reconnect a second later, so both are watched at once.
    await Promise.all(
      [owner.page, guestPage].map(page =>
        expect(
          page.getByText("Reconnecting: your changes won't reach the room"),
        ).toBeVisible(),
      ),
    );
    await expectLive(owner.page, 2);
    await expectLive(guestPage, 2);
    await expect(roomPlayButton(guestPage)).toBeVisible();
    await expect
      .poll(async () => Math.abs((await videoCurrentTime(guestPage)) - pausedAt))
      .toBeLessThan(1.5);

    // The owner closes the room; the guest is sent home.
    await owner.page.getByRole("button", { name: `Close watch room for ${MOVIE_TITLE}` }).click();
    await owner.page.getByRole("button", { name: "Close room" }).click();
    await expect(owner.page).toHaveURL(/\/$/);
    await expect(guestPage).toHaveURL(/\/$/);
    await expect(guestPage.getByText("Watch room closed")).toBeVisible();

    // The old link now leads nowhere. The server answers an unknown room
    // like one the viewer is not in, since room ids are guessable.
    guestIssues.ignore(roomRefused(roomId));
    await guestPage.goto(`/watch-rooms/${roomId}`);
    await expect(
      guestPage.getByText("This room no longer exists or you were not invited."),
    ).toBeVisible();
  });

  test("the owner swaps one invitee for another", async ({ browser }) => {
    const removed = await createGuest(owner.context.request, "Removed Guest");
    const added = await createGuest(owner.context.request, "Added Guest");
    const roomId = await createWatchRoom(owner.context.request, MOCK_MOVIE_ID, [removed.id]);

    const { page: removedPage } = await guestViewer(browser, removed);
    const { page: addedPage } = await guestViewer(browser, added);
    await owner.page.goto(`/watch-rooms/${roomId}`);
    // The removed guest arrives from Home, so its rooms list is cached with
    // the room in it.
    await removedPage.goto("/");
    const removedCard = removedPage.getByRole("article", { name: `Watch room: ${MOVIE_TITLE}` });
    await removedCard.getByRole("link", { name: `Join watch room for ${MOVIE_TITLE}` }).click();
    await expectLive(owner.page, 2);
    await expectLive(removedPage, 2);
    const roster = owner.page.getByRole("complementary");
    await expect(roster.getByText(removed.name)).toBeVisible();

    await owner.page.getByRole("button", { name: `Edit invitees for ${MOVIE_TITLE}` }).click();
    const dialog = owner.page.getByRole("dialog", { name: "Edit invitees" });
    const save = dialog.getByRole("button", { name: "Save invitees" });
    await expect(save).toBeDisabled();
    await dialog.getByRole("checkbox", { name: `Invite ${removed.name}` }).click();
    await dialog.getByRole("checkbox", { name: `Invite ${added.name}` }).click();
    // Home drops the room from the removed guest's cached list at once,
    // before the list's refetch could.
    const roomsRefetch = await gateRoute(removedPage, ROOMS_LIST);
    await save.click();
    await expect(dialog).toBeHidden();

    // The removed guest is put out at once; the owner's roster follows.
    await expect(removedPage).toHaveURL(url => url.pathname === "/");
    await expect(removedPage.getByText("No longer in this watch room")).toBeVisible();
    await expect.poll(roomsRefetch.requested).toBe(true);
    await expect(removedPage.getByRole("heading", { name: /continue watching/i })).toBeVisible();
    await expect(removedCard).toHaveCount(0);
    roomsRefetch.release();
    await expect(roster.getByText(added.name)).toBeVisible();
    await expect(roster.getByText(removed.name)).toBeHidden();
    await expect(owner.page.getByText("1 connected now")).toBeVisible();

    // The added guest finds the room on Home.
    await addedPage.goto("/");
    await expect(
      addedPage.getByRole("article", { name: `Watch room: ${MOVIE_TITLE}` }),
    ).toBeVisible();
  });

  test("a guest leaves a room for good", async ({ browser }) => {
    const guest = await createGuest(owner.context.request, "Leaving Guest");
    const roomId = await createWatchRoom(owner.context.request, MOCK_MOVIE_ID, [guest.id]);

    const { page: guestPage, issues: guestIssues } = await guestViewer(browser, guest);
    await owner.page.goto(`/watch-rooms/${roomId}`);
    // The guest arrives from Home, so its rooms list is cached with the room.
    await guestPage.goto("/");
    const card = guestPage.getByRole("article", { name: `Watch room: ${MOVIE_TITLE}` });
    await card.getByRole("link", { name: `Join watch room for ${MOVIE_TITLE}` }).click();
    await expectLive(guestPage, 2);
    await expectLive(owner.page, 2);

    // Only the owner closes the room; only a guest can leave it.
    await expect(
      owner.page.getByRole("button", { name: `Close watch room for ${MOVIE_TITLE}` }),
    ).toBeVisible();
    await expect(
      owner.page.getByRole("button", { name: `Leave watch room for ${MOVIE_TITLE}` }),
    ).toHaveCount(0);
    await expect(
      guestPage.getByRole("button", { name: `Close watch room for ${MOVIE_TITLE}` }),
    ).toHaveCount(0);

    // Leaving drops the room from that cached list at once, before the
    // list's refetch could.
    const roomsRefetch = await gateRoute(guestPage, ROOMS_LIST);
    await guestPage.getByRole("button", { name: `Leave watch room for ${MOVIE_TITLE}` }).click();
    await guestPage.getByRole("button", { name: "Leave room" }).click();
    await expect(guestPage).toHaveURL(url => url.pathname === "/");
    await expect(guestPage.getByText("Left watch room")).toBeVisible();
    await expect.poll(roomsRefetch.requested).toBe(true);
    await expect(guestPage.getByRole("heading", { name: /continue watching/i })).toBeVisible();
    await expect(card).toHaveCount(0);
    roomsRefetch.release();

    const roster = owner.page.getByRole("complementary");
    await expect(roster.getByText(guest.name)).toBeHidden();
    await expect(owner.page.getByText("1 connected now")).toBeVisible();

    // Opening the old link again finds nothing to join: the server refuses it.
    guestIssues.ignore(roomRefused(roomId));
    await guestPage.goto(`/watch-rooms/${roomId}`);
    await expect(
      guestPage.getByText("This room no longer exists or you were not invited."),
    ).toBeVisible();
  });

  test("someone who was never invited cannot open the room", async ({ browser }) => {
    const guest = await createGuest(owner.context.request, "Invited Guest");
    const outsider = await createGuest(owner.context.request, "Outsider");
    const roomId = await createWatchRoom(owner.context.request, MOCK_MOVIE_ID, [guest.id]);

    const { page } = await guestViewer(browser, outsider, roomRefused(roomId));
    await page.goto(`/watch-rooms/${roomId}`);

    await expect(page.getByText("Watch room unavailable")).toBeVisible();
    await expect(
      page.getByText("This room no longer exists or you were not invited."),
    ).toBeVisible();
  });

  // A handshake nobody answers fires neither open nor close: the page gives
  // up on it and tries again rather than waiting forever.
  test("a socket handshake that hangs is given up and tried again", async () => {
    const page = owner.page;
    const roomId = await createWatchRoom(owner.context.request, MOCK_MOVIE_ID, []);
    await page.goto(`/watch-rooms/${roomId}`);
    await expectLive(page, 1);

    // Chromium says so when a socket is closed before it opened.
    ownerIssues.ignore({
      ignoreConsole: (type, text) =>
        type === "warning" &&
        text.includes("WebSocket is closed before the connection is established."),
    });
    const stall = await owner.context.request.post(`/api/e2e/watch-rooms/${roomId}/stall`);
    expect(stall.ok()).toBe(true);
    const drop = await owner.context.request.post(`/api/e2e/watch-rooms/${roomId}/drop`);
    expect(drop.ok()).toBe(true);

    await expect(
      page.getByText("Reconnecting: your changes won't reach the room"),
    ).toBeVisible();
    // The connect timeout plus the next reconnect's backoff, in real time:
    // the handshake itself is real, so page.clock cannot stand in for it.
    await expectLive(page, 1, WATCH_ROOM_CONNECT_TIMEOUT_MS + 15_000);
  });

  // The socket cannot say why it was refused, so a reconnect asks to join
  // first, and that request hears the session ended.
  test("a viewer whose session ends in a room signs in again and comes back to it", async () => {
    const page = owner.page;
    const roomId = await createWatchRoom(owner.context.request, MOCK_MOVIE_ID, []);
    await page.goto(`/watch-rooms/${roomId}`);
    await expectLive(page, 1);

    ownerIssues.ignore({
      ignoreResponse: isSessionEndedResponse,
      ignoreConsole: (_type, text, message) => isApiResourceEcho(text, message),
    });
    await owner.context.clearCookies();
    const drop = await owner.context.request.post(`/api/e2e/watch-rooms/${roomId}/drop`);
    expect(drop.ok()).toBe(true);

    await expect(page).toHaveURL(url => url.pathname === "/login");
    await expect
      .poll(() => new URL(page.url()).searchParams.get("redirect"))
      .toBe(`/watch-rooms/${roomId}`);
    await expect(page.getByText("You've been signed out.")).toBeVisible();

    const login = await submitLogin(page, admin.email, admin.password);
    expect(login.status()).toBe(200);
    await expect(page).toHaveURL(url => url.pathname === `/watch-rooms/${roomId}`);
    await expectLive(page, 1);
  });

  // Fullscreen used to take the whole room page: the header stayed on top,
  // the video kept its card's width and the Exit button fell off the screen.
  test("fullscreen fills the screen with the player and keeps its controls on screen", async () => {
    const page = owner.page;
    const roomId = await createWatchRoom(owner.context.request, MOCK_MOVIE_ID, []);
    await page.goto(`/watch-rooms/${roomId}`);
    await expectLive(page, 1);

    await page.getByRole("button", { name: "Fullscreen" }).click();
    const exit = page.getByRole("button", { name: "Exit fullscreen" });
    await expect(exit).toHaveAttribute("aria-pressed", "true");

    // The player card is what went fullscreen: the video and its controls,
    // not the page's heading.
    const fullscreen = page.locator(":fullscreen");
    await expect(fullscreen).toHaveCount(1);
    expect(await fullscreen.evaluate(element => element.tagName)).toBe("SECTION");
    const video = fullscreen.locator("video");
    const transport = fullscreen.getByRole("group", { name: "Playback controls" });
    await expect(video).toHaveCount(1);
    await expect(transport.getByRole("button", { name: "Exit fullscreen" })).toBeVisible();
    await expect(fullscreen.getByRole("heading", { level: 1 })).toHaveCount(0);

    // The card fills the screen; the video takes all the height the
    // transport row leaves it, and Exit stays on screen. Polled because the
    // card resizes after the fullscreen change is announced.
    const viewport = page.viewportSize()!;
    const layout = async () => {
      const [videoBox, transportBox, exitBox] = await Promise.all([
        video.boundingBox(),
        transport.boundingBox(),
        exit.boundingBox(),
      ]);
      return {
        videoLeft: videoBox!.x,
        videoTop: videoBox!.y,
        videoWidth: videoBox!.width,
        fillsHeight:
          videoBox!.y + videoBox!.height > viewport.height / 2 &&
          Math.abs(transportBox!.y - (videoBox!.y + videoBox!.height)) <= 1,
        exitOnScreen: exitBox!.y + exitBox!.height <= viewport.height,
      };
    };
    await expect.poll(layout).toEqual({
      videoLeft: 0,
      videoTop: 0,
      videoWidth: viewport.width,
      fillsHeight: true,
      exitOnScreen: true,
    });

    await exit.click();
    await expect(page.getByRole("button", { name: "Fullscreen" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    expect(await page.evaluate(() => document.fullscreenElement)).toBeNull();
    await expect(page.getByRole("heading", { name: "People in this room" })).toBeVisible();
  });
});
