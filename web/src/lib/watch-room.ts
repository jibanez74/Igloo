import type { WatchRoomServerEventType } from "@/types";

export function watchRoomStreamUrl(roomId: number, playbackMode: string) {
  if (playbackMode === "direct") {
    return `/api/watch-rooms/${roomId}/stream`;
  }

  return `/api/watch-rooms/${roomId}/hls/playlist.m3u8`;
}

export function watchRoomWebSocketUrl(roomId: number) {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${window.location.host}/api/watch-rooms/${roomId}/ws`;
}

export function watchRoomAnnouncement(
  event: WatchRoomServerEventType,
  title: string,
): string | undefined {
  switch (event.type) {
    case "playback_changed":
      if (!event.playback) return undefined;
      if (event.playback.paused) {
        return `Paused ${title}`;
      }
      return `Playing ${title}`;
    case "member_joined":
      return event.member ? `${event.member.name} joined the room` : undefined;
    case "member_left":
      return event.member ? `${event.member.name} left the room` : undefined;
    default:
      return undefined;
  }
}
