// The OS media controls (lock screen, headset buttons, the browser's media hub)
// read one global navigator.mediaSession. The music player and the video
// surfaces both publish to it, so it has one owner at a time: the player that
// last claimed it. A claim replaces the whole action set, so a video never
// inherits music's previous/next/stop. A release clears the session only if
// the releasing player still owns it, so a stale cleanup never wipes the
// player that took over. Music watches the owner and claims the session back
// once a video lets it go.

export const MEDIA_SESSION_ACTIONS = [
  "play",
  "pause",
  "stop",
  "previoustrack",
  "nexttrack",
  "seekbackward",
  "seekforward",
  "seekto",
] as const;

type MediaSessionActionName = (typeof MEDIA_SESSION_ACTIONS)[number];

export type MediaSessionHandlers = Partial<
  Record<MediaSessionActionName, MediaSessionActionHandler>
>;

let owner: symbol | null = null;
const listeners = new Set<() => void>();

function mediaSessionSupported() {
  return typeof navigator !== "undefined" && "mediaSession" in navigator;
}

function setOwner(next: symbol | null) {
  if (owner === next) return;
  owner = next;
  for (const listener of listeners) listener();
}

function setHandler(
  action: MediaSessionActionName,
  handler: MediaSessionActionHandler | null,
) {
  try {
    navigator.mediaSession.setActionHandler(action, handler);
  } catch {
    // A browser that does not support an action (older Safari and "stop")
    // throws instead of ignoring it.
  }
}

/** The player that owns the session, or null. For useSyncExternalStore. */
export function getMediaSessionOwner() {
  return owner;
}

export function subscribeMediaSessionOwner(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

export function ownsMediaSession(token: symbol) {
  return owner === token;
}

/**
 * Makes `token` the owner and publishes its metadata and its complete action
 * set; every action it does not handle is cleared.
 */
export function claimMediaSession(
  token: symbol,
  metadata: MediaMetadata | null,
  handlers: MediaSessionHandlers,
) {
  if (!mediaSessionSupported()) return;

  navigator.mediaSession.metadata = metadata;
  for (const action of MEDIA_SESSION_ACTIONS) {
    setHandler(action, handlers[action] ?? null);
  }
  setOwner(token);
}

/** Replaces the metadata if `token` still owns the session. */
export function updateMediaSessionMetadata(
  token: symbol,
  metadata: MediaMetadata | null,
) {
  if (owner !== token || !mediaSessionSupported()) return;

  navigator.mediaSession.metadata = metadata;
}

/**
 * Clears the session if `token` still owns it, timeline included, so the next
 * owner never shows the previous one's position; otherwise does nothing.
 */
export function releaseMediaSession(token: symbol) {
  if (owner !== token || !mediaSessionSupported()) return;

  navigator.mediaSession.metadata = null;
  navigator.mediaSession.playbackState = "none";
  for (const action of MEDIA_SESSION_ACTIONS) {
    setHandler(action, null);
  }
  try {
    // No argument clears the position state.
    navigator.mediaSession.setPositionState?.();
  } catch {
    // Some browsers expose Media Session but reject position updates.
  }
  setOwner(null);
}
