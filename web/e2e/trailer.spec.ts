import { expect, test, type Page } from "@playwright/test";
import { trackBrowserIssues } from "./e2e-browser-issues";
import { expectPageHasNoHorizontalScroll } from "./e2e-layout";
import { apiURL, readE2EEnv, type E2EEnv } from "./e2e-env";

async function login(page: Page, env: E2EEnv) {
  const response = await page.context().request.post(
    apiURL(env, "/api/auth/login"),
    {
      data: {
        email: env.email,
        password: env.password,
      },
      failOnStatusCode: false,
    },
  );
  expect(response.status()).toBe(200);
}

async function mockYouTubePlayer(page: Page) {
  await page.addInitScript(() => {
    type FakePlayerEvent = { target: FakePlayer };
    type FakePlayerStateEvent = { data: number; target: FakePlayer };
    type FakePlayerEvents = {
      onReady?: (event: FakePlayerEvent) => void;
      onStateChange?: (event: FakePlayerStateEvent) => void;
    };

    class FakePlayer {
      currentTime = 0;
      duration = 148;
      muted = false;
      volume = 80;
      state = window.YT.PlayerState.CUED;
      events: FakePlayerEvents;

      constructor(_id: string, options: YT.PlayerOptions) {
        this.events = options.events ?? {};
        window.setTimeout(() => {
          this.events.onReady?.({ target: this });
        }, 0);
      }

      getCurrentTime() {
        return this.currentTime;
      }

      getDuration() {
        return this.duration;
      }

      getVolume() {
        return this.volume;
      }

      isMuted() {
        return this.muted;
      }

      getPlayerState() {
        return this.state;
      }

      playVideo() {
        this.state = window.YT.PlayerState.PLAYING;
        this.events.onStateChange?.({
          data: this.state,
          target: this,
        });
      }

      pauseVideo() {
        this.state = window.YT.PlayerState.PAUSED;
        this.events.onStateChange?.({
          data: this.state,
          target: this,
        });
      }

      seekTo(seconds: number) {
        this.currentTime = Math.max(0, Math.min(this.duration, seconds));
      }

      setVolume(volume: number) {
        this.volume = Math.max(0, Math.min(100, volume));
      }

      mute() {
        this.muted = true;
      }

      unMute() {
        this.muted = false;
      }

      destroy() {}
    }

    window.YT = {
      Player: FakePlayer as unknown as typeof YT.Player,
      PlayerState: {
        UNSTARTED: -1,
        ENDED: 0,
        PLAYING: 1,
        PAUSED: 2,
        BUFFERING: 3,
        CUED: 5,
      },
      PlayerError: {
        INVALID_PARAM: 2,
        HTML5_ERROR: 5,
        NOT_FOUND: 100,
        NOT_ALLOWED: 101,
        NOT_ALLOWED_DISGUISE: 150,
      },
    };
  });
}

async function expectTrailerChrome(page: Page) {
  await expect(page.getByRole("dialog", { name: "Trailer" })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Close trailer (Escape)" }),
  ).toBeVisible();
  await expect(
    page.getByRole("slider", { name: "Seek through trailer" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Play (Space or K)" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Rewind 10 seconds (J or Left Arrow)" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Forward 10 seconds (L or Right Arrow)" }),
  ).toBeVisible();
  await expect(page.getByRole("button", { name: "Mute (M)" })).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Fullscreen (F)" }),
  ).toBeVisible();
  await expectPageHasNoHorizontalScroll(page);
}

test.describe("Trailer playback chrome", () => {
  for (const { name, viewport, reducedMotion } of [
    {
      name: "desktop",
      viewport: { width: 1440, height: 900 },
      reducedMotion: "no-preference" as const,
    },
    {
      name: "mobile reduced motion",
      viewport: { width: 390, height: 844 },
      reducedMotion: "reduce" as const,
    },
  ]) {
    test(`renders labelled trailer controls on ${name}`, async ({ page }) => {
      const env = readE2EEnv();
      const browserIssues = trackBrowserIssues(page);

      await login(page, env);
      await mockYouTubePlayer(page);
      await page.emulateMedia({ reducedMotion });
      await page.setViewportSize(viewport);
      await page.goto(
        apiURL(
          env,
          "/trailer?videoKey=signal-fire-trailer&returnTo=/movies/101",
        ),
        { waitUntil: "networkidle" },
      );

      await expectTrailerChrome(page);
      browserIssues.assertClean();
    });
  }
});
