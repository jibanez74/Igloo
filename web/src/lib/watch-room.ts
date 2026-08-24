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

export function watchRoomWebSocketUrl(roomId: number) {
  const baseUrl = new URL(window.location.origin);
  baseUrl.protocol = window.location.protocol === "https:" ? "wss:" : "ws:";

  if (import.meta.env.DEV) {
    baseUrl.port = "8080";
  }

  return new URL(`/api/watch-rooms/${roomId}/ws`, baseUrl).toString();
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
