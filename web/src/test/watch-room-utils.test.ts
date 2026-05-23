import { describe, expect, it } from "vitest";
import {
  watchRoomAnnouncement,
  watchRoomStreamUrl,
  watchRoomWebSocketUrl,
} from "@/lib/watch-room";
import type { WatchRoomServerEventType } from "@/types";

describe("watch-room utilities", () => {
  it("builds direct and HLS stream URLs from the room playback mode", () => {
    expect(watchRoomStreamUrl(7, "direct")).toBe("/api/watch-rooms/7/stream");
    expect(watchRoomStreamUrl(7, "720p_3mbps")).toBe(
      "/api/watch-rooms/7/hls/playlist.m3u8",
    );
  });

  it("builds websocket URLs with the current browser protocol", () => {
    const url = new URL(watchRoomWebSocketUrl(42));

    expect(url.protocol).toBe(
      window.location.protocol === "https:" ? "wss:" : "ws:",
    );
    expect(url.pathname).toBe("/api/watch-rooms/42/ws");
    if (import.meta.env.DEV) {
      expect(url.port).toBe("8080");
    }
  });

  it("announces meaningful realtime events", () => {
    const playback: WatchRoomServerEventType = {
      type: "playback_changed",
      room_id: 7,
      playback: {
        paused: false,
        position_sec: 12,
        updated_at: "2026-04-14T12:00:00Z",
      },
    };

    expect(watchRoomAnnouncement(playback, "Arrival")).toBe("Playing Arrival");
    expect(
      watchRoomAnnouncement(
        {
          ...playback,
          playback: {
            ...playback.playback!,
            paused: true,
          },
        },
        "Arrival",
      ),
    ).toBe("Paused Arrival");
    expect(
      watchRoomAnnouncement(
        {
          type: "member_joined",
          room_id: 7,
          member: {
            id: 2,
            name: "Dana Scully",
            avatar: null,
          },
        },
        "Arrival",
      ),
    ).toBe("Dana Scully joined the room");
    expect(
      watchRoomAnnouncement(
        {
          type: "member_left",
          room_id: 7,
          member: {
            id: 2,
            name: "Dana Scully",
            avatar: null,
          },
        },
        "Arrival",
      ),
    ).toBe("Dana Scully left the room");
    expect(
      watchRoomAnnouncement({ type: "pong", room_id: 7 }, "Arrival"),
    ).toBeUndefined();
  });
});
