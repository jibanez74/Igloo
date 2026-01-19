/**
 * YouTube IFrame Player API TypeScript definitions
 * @see https://developers.google.com/youtube/iframe_api_reference
 */

declare global {
  interface Window {
    YT: typeof YT;
    onYouTubeIframeAPIReady: () => void;
  }
}

// eslint-disable-next-line @typescript-eslint/no-unused-vars
declare namespace YT {
  /** Player state constants */
  enum PlayerState {
    UNSTARTED = -1,
    ENDED = 0,
    PLAYING = 1,
    PAUSED = 2,
    BUFFERING = 3,
    CUED = 5,
  }

  /** Error codes */
  enum PlayerError {
    INVALID_PARAM = 2,
    HTML5_ERROR = 5,
    NOT_FOUND = 100,
    NOT_ALLOWED = 101,
    NOT_ALLOWED_DISGUISE = 150,
  }

  /** Player options for initialization */
  interface PlayerOptions {
    height?: string | number;
    width?: string | number;
    videoId?: string;
    playerVars?: PlayerVars;
    events?: PlayerEvents;
  }

  /** Player variables for customizing the player */
  interface PlayerVars {
    autoplay?: 0 | 1;
    cc_lang_pref?: string;
    cc_load_policy?: 0 | 1;
    color?: "red" | "white";
    controls?: 0 | 1;
    disablekb?: 0 | 1;
    enablejsapi?: 0 | 1;
    end?: number;
    fs?: 0 | 1;
    hl?: string;
    iv_load_policy?: 1 | 3;
    list?: string;
    listType?: "playlist" | "search" | "user_uploads";
    loop?: 0 | 1;
    modestbranding?: 0 | 1;
    origin?: string;
    playlist?: string;
    playsinline?: 0 | 1;
    rel?: 0 | 1;
    start?: number;
    widget_referrer?: string;
  }

  /** Event handlers */
  interface PlayerEvents {
    onReady?: (event: PlayerEvent) => void;
    onStateChange?: (event: OnStateChangeEvent) => void;
    onPlaybackQualityChange?: (event: OnPlaybackQualityChangeEvent) => void;
    onPlaybackRateChange?: (event: OnPlaybackRateChangeEvent) => void;
    onError?: (event: OnErrorEvent) => void;
    onApiChange?: (event: PlayerEvent) => void;
  }

  /** Base event */
  interface PlayerEvent {
    target: Player;
  }

  /** State change event */
  interface OnStateChangeEvent extends PlayerEvent {
    data: PlayerState;
  }

  /** Playback quality change event */
  interface OnPlaybackQualityChangeEvent extends PlayerEvent {
    data: string;
  }

  /** Playback rate change event */
  interface OnPlaybackRateChangeEvent extends PlayerEvent {
    data: number;
  }

  /** Error event */
  interface OnErrorEvent extends PlayerEvent {
    data: PlayerError;
  }

  /** The YouTube Player class */
  class Player {
    constructor(elementId: string | HTMLElement, options: PlayerOptions);

    // Playback controls
    playVideo(): void;
    pauseVideo(): void;
    stopVideo(): void;
    seekTo(seconds: number, allowSeekAhead?: boolean): void;

    // Volume controls
    mute(): void;
    unMute(): void;
    isMuted(): boolean;
    setVolume(volume: number): void;
    getVolume(): number;

    // Player size
    setSize(width: number, height: number): void;

    // Playback rate
    getPlaybackRate(): number;
    setPlaybackRate(suggestedRate: number): void;
    getAvailablePlaybackRates(): number[];

    // Playlist controls
    nextVideo(): void;
    previousVideo(): void;
    playVideoAt(index: number): void;

    // Video info
    getDuration(): number;
    getCurrentTime(): number;
    getVideoLoadedFraction(): number;
    getPlayerState(): PlayerState;
    getVideoUrl(): string;
    getVideoEmbedCode(): string;

    // Playlist info
    getPlaylist(): string[];
    getPlaylistIndex(): number;

    // Player state
    destroy(): void;
    getIframe(): HTMLIFrameElement;

    // Video loading
    cueVideoById(videoId: string, startSeconds?: number): void;
    loadVideoById(videoId: string, startSeconds?: number): void;
    cueVideoByUrl(mediaContentUrl: string, startSeconds?: number): void;
    loadVideoByUrl(mediaContentUrl: string, startSeconds?: number): void;
  }
}

export {};
