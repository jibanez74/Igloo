import { expect, test, type Browser, type BrowserContext, type Page } from "@playwright/test";

import { MOVIE_SEEK_STEP_SEC } from "../src/lib/constants";
import { loginViaApi } from "./e2e-auth";
import { trackBrowserIssues } from "./e2e-browser-issues";
import { readE2EEnv, requireMockApi, type Credentials } from "./e2e-env";
import { MOCK_MOVIE_ID } from "./fixtures/movies";
import {
  createGuest,
  createWatchRoom,
  fakeWatchRoomMedia,
  videoCurrentTime,
  type E2EGuest,
} from "./watch-room-e2e";

// Watch rooms against the mock API's rooms and socket (mock-watch-rooms.ts):
// every flow a member can take, two or three browsers at a time. The real
// server runs the same flows in watch-room.spec.ts.

requireMockApi();

const MOVIE_TITLE = "Signal Fire";
const admin = readE2EEnv();

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

async function expectLive(page: Page, connected: number) {
  await expect(page.getByText("Realtime sync connected")).toBeVisible();
  await expect(page.getByText(`${connected} connected now`)).toBeVisible();
}

const playButton = (page: Page) => page.getByRole("button", { name: "Play playback" });
const pauseButton = (page: Page) => page.getByRole("button", { name: "Pause playback" });

test.describe("Watch rooms", () => {
  let owner: Viewer;
  const others: Viewer[] = [];

  test.beforeEach(async ({ browser }) => {
    owner = await openViewer(browser);
  });

  test.afterEach(async () => {
    for (const viewer of [owner, ...others.splice(0)]) {
      await viewer.context.close();
    }
  });

  async function guestViewer(browser: Browser, guest: E2EGuest) {
    const viewer = await openViewer(browser, guest);
    others.push(viewer);
    return viewer;
  }

  test("an owner opens a room from the movie, a guest joins from Home, and both share playback", async ({
    browser,
  }) => {
    const guest = await createGuest(owner.context.request, "Couch Guest");
    const ownerIssues = trackBrowserIssues(owner.page);

    await owner.page.goto(`/movies/${MOCK_MOVIE_ID}`);
    await owner.page.getByRole("button", { name: "More options" }).click();
    await owner.page.getByRole("menuitem", { name: "Watch Together" }).click();
    const dialog = owner.page.getByRole("dialog", { name: "Watch together" });
    await dialog.getByRole("checkbox", { name: `Invite ${guest.name}` }).click();
    await dialog.getByRole("button", { name: "Create and join room" }).click();
    await expect(owner.page).toHaveURL(/\/watch-rooms\/\d+$/);
    await expectLive(owner.page, 1);

    const { page: guestPage } = await guestViewer(browser, guest);
    const guestIssues = trackBrowserIssues(guestPage);
    await guestPage.goto("/");
    const card = guestPage.getByRole("article", { name: `Watch room: ${MOVIE_TITLE}` });
    await card.getByRole("link", { name: `Join watch room for ${MOVIE_TITLE}` }).click();
    await expectLive(guestPage, 2);
    await expectLive(owner.page, 2);

    // Play and seek from the owner, pause from the guest.
    await playButton(owner.page).click();
    await expect(pauseButton(guestPage)).toBeVisible();
    await owner.page
      .getByRole("button", { name: `Fast-forward ${MOVIE_SEEK_STEP_SEC} seconds` })
      .click();
    await expect
      .poll(() => videoCurrentTime(guestPage))
      .toBeGreaterThanOrEqual(MOVIE_SEEK_STEP_SEC - 0.5);
    await pauseButton(guestPage).click();
    await expect(playButton(owner.page)).toBeVisible();
    const pausedAt = await videoCurrentTime(owner.page);

    // A network blip: both pages say so, come back, and keep the room's place.
    const roomId = Number(owner.page.url().split("/").at(-1));
    const drop = await owner.context.request.post(`/api/e2e/watch-rooms/${roomId}/drop`);
    expect(drop.ok()).toBe(true);
    for (const page of [owner.page, guestPage]) {
      await expect(
        page.getByText("Reconnecting: your changes won't reach the room"),
      ).toBeVisible();
    }
    await expectLive(owner.page, 2);
    await expectLive(guestPage, 2);
    await expect(playButton(guestPage)).toBeVisible();
    expect(Math.abs((await videoCurrentTime(guestPage)) - pausedAt)).toBeLessThan(1.5);

    // The owner closes the room; the guest is sent home.
    await owner.page.getByRole("button", { name: `Close watch room for ${MOVIE_TITLE}` }).click();
    await owner.page.getByRole("button", { name: "Close room" }).click();
    await expect(owner.page).toHaveURL(/\/$/);
    await expect(guestPage).toHaveURL(/\/$/);
    await expect(guestPage.getByText("Watch room closed")).toBeVisible();

    await ownerIssues.assertClean();
    await guestIssues.assertClean();
  });

  test("the owner swaps one invitee for another", async ({ browser }) => {
    const removed = await createGuest(owner.context.request, "Removed Guest");
    const added = await createGuest(owner.context.request, "Added Guest");
    const roomId = await createWatchRoom(owner.context.request, MOCK_MOVIE_ID, [removed.id]);

    const { page: removedPage } = await guestViewer(browser, removed);
    const { page: addedPage } = await guestViewer(browser, added);
    await owner.page.goto(`/watch-rooms/${roomId}`);
    await removedPage.goto(`/watch-rooms/${roomId}`);
    await expectLive(owner.page, 2);
    await expectLive(removedPage, 2);

    await owner.page.getByRole("button", { name: `Edit invitees for ${MOVIE_TITLE}` }).click();
    const dialog = owner.page.getByRole("dialog", { name: "Edit invitees" });
    const save = dialog.getByRole("button", { name: "Save invitees" });
    await expect(save).toBeDisabled();
    await dialog.getByRole("checkbox", { name: `Invite ${removed.name}` }).click();
    await dialog.getByRole("checkbox", { name: `Invite ${added.name}` }).click();
    await save.click();
    await expect(dialog).toBeHidden();

    // The removed guest is put out at once; the owner's roster follows.
    await expect(removedPage).toHaveURL(/\/$/);
    await expect(removedPage.getByText("No longer in this watch room")).toBeVisible();
    const roster = owner.page.getByRole("complementary");
    await expect(roster.getByText(added.name)).toBeVisible();
    await expect(roster.getByText(removed.name)).toBeHidden();
    await expect(owner.page.getByText("1 connected now")).toBeVisible();

    // The added guest finds the room on Home; the removed one does not.
    await addedPage.goto("/");
    await expect(
      addedPage.getByRole("article", { name: `Watch room: ${MOVIE_TITLE}` }),
    ).toBeVisible();
    await removedPage.goto("/");
    await expect(removedPage.getByRole("heading", { name: /continue watching/i })).toBeVisible();
    await expect(
      removedPage.getByRole("article", { name: `Watch room: ${MOVIE_TITLE}` }),
    ).toHaveCount(0);
  });

  test("a guest leaves a room for good", async ({ browser }) => {
    const guest = await createGuest(owner.context.request, "Leaving Guest");
    const roomId = await createWatchRoom(owner.context.request, MOCK_MOVIE_ID, [guest.id]);

    const { page: guestPage } = await guestViewer(browser, guest);
    await owner.page.goto(`/watch-rooms/${roomId}`);
    await guestPage.goto(`/watch-rooms/${roomId}`);
    await expectLive(guestPage, 2);

    await guestPage.getByRole("button", { name: `Leave watch room for ${MOVIE_TITLE}` }).click();
    await guestPage.getByRole("button", { name: "Leave room" }).click();
    await expect(guestPage).toHaveURL(/\/$/);
    await expect(guestPage.getByText("Left watch room")).toBeVisible();
    await expect(
      guestPage.getByRole("article", { name: `Watch room: ${MOVIE_TITLE}` }),
    ).toHaveCount(0);

    const roster = owner.page.getByRole("complementary");
    await expect(roster.getByText(guest.name)).toBeHidden();
    await expect(owner.page.getByText("1 connected now")).toBeVisible();

    // Opening the old link again finds nothing to join.
    await guestPage.goto(`/watch-rooms/${roomId}`);
    await expect(
      guestPage.getByText("This room no longer exists or you were not invited."),
    ).toBeVisible();
  });

  test("someone who was never invited cannot open the room", async ({ browser }) => {
    const guest = await createGuest(owner.context.request, "Invited Guest");
    const outsider = await createGuest(owner.context.request, "Outsider");
    const roomId = await createWatchRoom(owner.context.request, MOCK_MOVIE_ID, [guest.id]);

    const { page } = await guestViewer(browser, outsider);
    const issues = trackBrowserIssues(page, {
      ignoreResponse: response =>
        response.status() === 403 && response.url().includes(`/api/watch-rooms/${roomId}`),
      ignoreConsole: (_type, text) => text.includes("status of 403"),
    });
    await page.goto(`/watch-rooms/${roomId}`);

    await expect(page.getByText("Watch room unavailable")).toBeVisible();
    await expect(
      page.getByText("This room no longer exists or you were not invited."),
    ).toBeVisible();
    await issues.assertClean();
  });

  // Fullscreen used to take the whole room page: the header stayed on top,
  // the video kept its card's width and the Exit button fell off the screen.
  test("fullscreen fills the screen with the player and keeps its controls on screen", async () => {
    const page = owner.page;
    const issues = trackBrowserIssues(page);
    const roomId = await createWatchRoom(owner.context.request, MOCK_MOVIE_ID, []);
    await page.goto(`/watch-rooms/${roomId}`);
    await expectLive(page, 1);

    await page.getByRole("button", { name: "Fullscreen" }).click();
    const exit = page.getByRole("button", { name: "Exit fullscreen" });
    await expect(exit).toHaveAttribute("aria-pressed", "true");

    const layout = await page.evaluate(() => {
      const fullscreen = document.fullscreenElement;
      const video = document.querySelector("video")!.getBoundingClientRect();
      const exitButton = document.querySelector('button[aria-label="Exit fullscreen"]')!;
      const button = exitButton.getBoundingClientRect();
      // The transport row is the bordered block under the video that holds Exit.
      const transport = exitButton.closest(".border-t")!.getBoundingClientRect();
      return {
        tag: fullscreen?.tagName ?? null,
        holdsVideo: !!fullscreen?.querySelector("video"),
        holdsExit: !!fullscreen?.querySelector('button[aria-label="Exit fullscreen"]'),
        holdsHeading: !!fullscreen?.querySelector("h1"),
        videoLeft: video.left,
        videoWidth: video.width,
        videoTop: video.top,
        videoBottom: video.bottom,
        transportTop: transport.top,
        exitBottom: button.bottom,
        innerWidth: window.innerWidth,
        innerHeight: window.innerHeight,
      };
    });
    expect(layout).toMatchObject({
      tag: "SECTION",
      holdsVideo: true,
      holdsExit: true,
      holdsHeading: false,
      videoLeft: 0,
      videoWidth: layout.innerWidth,
      videoTop: 0,
    });
    // The video takes all the height the transport row leaves it.
    expect(layout.videoBottom).toBeGreaterThan(layout.innerHeight / 2);
    expect(Math.abs(layout.transportTop - layout.videoBottom)).toBeLessThanOrEqual(1);
    expect(layout.exitBottom).toBeLessThanOrEqual(layout.innerHeight);

    await exit.click();
    await expect(page.getByRole("button", { name: "Fullscreen" })).toHaveAttribute(
      "aria-pressed",
      "false",
    );
    expect(await page.evaluate(() => document.fullscreenElement)).toBeNull();
    await expect(page.getByRole("heading", { name: "People in this room" })).toBeVisible();
    await issues.assertClean();
  });
});
