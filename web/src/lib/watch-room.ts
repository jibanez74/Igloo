import type { WatchRoomServerEventType } from "@/types";
import { WATCH_ROOM_EVENT_TYPES } from "@/lib/constants";

/**
 * `reloadKey` is a client-side remount trigger, not a server parameter: the
 * room manifest handler ignores unknown query values. Bumping it changes the
 * `src` identity so the player rebuilds its hls.js instance after a lost
 * session, mirroring the `reload` parameter personal playback uses.
 */
export function watchRoomStreamUrl(
  roomId: number,
  playbackMode: string,
  reloadKey = 0,
) {
  if (playbackMode === "direct") {
    return `/api/watch-rooms/${roomId}/stream`;
  }

  const query = reloadKey > 0 ? `?reload=${reloadKey}` : "";

  return `/api/watch-rooms/${roomId}/hls/playlist.m3u8${query}`;
}

// The page's own origin, as every API call uses: in development Vite proxies
// /api, the socket included, to the Go server.
export function watchRoomWebSocketUrl(roomId: number) {
  const baseUrl = new URL(window.location.origin);
  baseUrl.protocol = window.location.protocol === "https:" ? "wss:" : "ws:";

  return new URL(`/api/watch-rooms/${roomId}/ws`, baseUrl).toString();
}

/**
 * Whether the room's stream answers 409: the tracks the room pinned no longer
 * match its file after a rescan. Asked only after the player failed, because
 * the player cannot see the status. A direct room takes HEAD; an HLS room's
 * manifest only answers GET, which also refreshes its session.
 */
export async function watchRoomStreamConflicted(
  streamUrl: string,
  playbackMode: string,
): Promise<boolean> {
  try {
    const response = await fetch(streamUrl, {
      method: playbackMode === "direct" ? "HEAD" : "GET",
      credentials: "same-origin",
      cache: "no-store",
    });
    void response.body?.cancel();
    return response.status === 409;
  } catch {
    return false;
  }
}

export function watchRoomAnnouncement(
  event: WatchRoomServerEventType,
  title: string,
): string | undefined {
  switch (event.type) {
    case WATCH_ROOM_EVENT_TYPES.PLAYBACK_CHANGED:
      if (!event.playback) return undefined;
      if (event.playback.paused) {
        return `Paused ${title}`;
      }
      return `Playing ${title}`;
    case WATCH_ROOM_EVENT_TYPES.MEMBER_JOINED:
      return event.member ? `${event.member.name} joined the room` : undefined;
    case WATCH_ROOM_EVENT_TYPES.MEMBER_LEFT:
      return event.member ? `${event.member.name} left the room` : undefined;
    default:
      return undefined;
  }
}
