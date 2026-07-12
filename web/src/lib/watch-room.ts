import type { WatchRoomServerEventType } from "@/types";
import { WATCH_ROOM_EVENT_TYPES } from "@/lib/constants";

export function watchRoomStreamUrl(roomId: number, playbackMode: string) {
  if (playbackMode === "direct") {
    return `/api/watch-rooms/${roomId}/stream`;
  }

  return `/api/watch-rooms/${roomId}/hls/playlist.m3u8`;
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
