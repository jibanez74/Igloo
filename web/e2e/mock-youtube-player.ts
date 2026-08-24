import { type Page } from "@playwright/test";

export type MockYouTubePlayerOptions = {
  failFirstLoad?: boolean;
};

/**
 * Installs a fake `window.YT` IFrame API before the app boots so trailer/extra
 * video playback can be exercised without loading the real YouTube player.
 *
 * `useYouTubePlayer` short-circuits the external `iframe_api` script when
 * `window.YT.Player` already exists, so this `addInitScript` mock also survives
 * client-side (SPA) navigations such as opening the trailer dialog from the
 * movie details page.
 */
export async function mockYouTubePlayer(
  page: Page,
  { failFirstLoad = false }: MockYouTubePlayerOptions = {},
) {
  await page.addInitScript(({ failFirstLoad }) => {
    let playerCreations = 0;

    class FakePlayer {
      currentTime = 0;
      duration = 148;
      muted = false;
      volume = 80;
      state = window.YT.PlayerState.CUED;
      events: YT.PlayerEvents;

      constructor(_id: string, options: YT.PlayerOptions) {
        this.events = options.events ?? {};
        playerCreations += 1;

        window.setTimeout(() => {
          const target = this as unknown as YT.Player;

          if (failFirstLoad && playerCreations === 1) {
            this.events.onError?.({
              data: window.YT.PlayerError.HTML5_ERROR,
              target,
            });
            return;
          }

          this.events.onReady?.({ target });
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
          target: this as unknown as YT.Player,
        });
      }

      pauseVideo() {
        this.state = window.YT.PlayerState.PAUSED;
        this.events.onStateChange?.({
          data: this.state,
          target: this as unknown as YT.Player,
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
  }, { failFirstLoad });
}
