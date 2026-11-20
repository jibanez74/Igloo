import type { IncomingMessage, Server, ServerResponse } from "node:http";
import type { Duplex } from "node:stream";
import { WebSocketServer, type WebSocket } from "ws";
import type {
  CreateWatchRoomRequestType,
  WatchRoomDetailType,
  WatchRoomMemberType,
  WatchRoomServerEventType,
  WatchRoomType,
} from "../src/types";
import { WATCH_ROOM_CLIENT_EVENT_TYPES, WATCH_ROOM_EVENT_TYPES } from "../src/lib/constants";

// Watch rooms for the stateful mock API: the HTTP routes and the room socket,
// with the Go server's rules (docs/openapi.json, server/cmd/api/watch_room_*.go)
// where a spec can see them — membership answers, the owner-only edits, the
// events each change sends, and every command echoed to its sender too. Media
// is not real: the stream answers an empty MP4 and the specs fake the element.

type RoomUser = WatchRoomMemberType & { email: string };

type PlaybackState = { paused: boolean; positionSec: number; updatedAt: number };

type Room = {
  id: number;
  ownerId: number;
  movieId: number;
  mode: string;
  audioTrack: number;
  subtitleTrack: number | null;
  memberIds: number[];
  createdAt: string;
  playback: PlaybackState;
};

type RoomSocket = { ws: WebSocket; roomId: number; userId: number };

export type WatchRoomDeps = {
  currentUserId: (request: IncomingMessage) => number | null;
  findUser: (id: number) => RoomUser | null;
  listUsers: () => RoomUser[];
  findMovie: (id: number) => { title: string; poster: string | null } | null;
  sendSuccess: (response: ServerResponse, data?: unknown, status?: number) => void;
  sendFailure: (response: ServerResponse, status: number, message: string) => void;
  readJSONBody: (request: IncomingMessage) => Promise<Record<string, unknown>>;
};

// Messages the Go handlers answer with.
const ACCESS_DENIED = "access denied";
const ROOM_NOT_FOUND = "room not found";

export function createMockWatchRooms(deps: WatchRoomDeps) {
  const rooms = new Map<number, Room>();
  const sockets = new Set<RoomSocket>();
  // Rooms whose next socket upgrade is left unanswered, and the raw sockets
  // held that way (closed on reset).
  const stallNextUpgrade = new Set<number>();
  const stalledSockets = new Set<Duplex>();
  let nextRoomId = 1;
  const wss = new WebSocketServer({ noServer: true });

  const summary = (id: number): WatchRoomMemberType => {
    const user = deps.findUser(id);
    return { id, name: user?.name ?? "Unknown", avatar: user?.avatar ?? null };
  };

  const positionAt = (state: PlaybackState, now: number) =>
    state.paused ? state.positionSec : state.positionSec + (now - state.updatedAt) / 1000;

  const playbackSnapshot = (room: Room) => {
    const now = Date.now();
    return {
      paused: room.playback.paused,
      position_sec: positionAt(room.playback, now),
      updated_at: new Date(now).toISOString(),
    };
  };

  const connectedIds = (roomId: number) =>
    [...new Set([...sockets].filter(s => s.roomId === roomId).map(s => s.userId))].sort(
      (a, b) => a - b,
    );

  const send = (socket: RoomSocket, event: WatchRoomServerEventType) => {
    socket.ws.send(JSON.stringify(event));
  };

  const broadcast = (
    roomId: number,
    event: WatchRoomServerEventType,
    except?: RoomSocket,
  ) => {
    for (const socket of sockets) {
      if (socket.roomId === roomId && socket !== except) send(socket, event);
    }
  };

  const listItem = (room: Room, viewerId: number): WatchRoomType => {
    const movie = deps.findMovie(room.movieId);
    return {
      id: room.id,
      movie_id: room.movieId,
      movie_title: movie?.title ?? "",
      movie_poster: movie?.poster ?? null,
      owner: summary(room.ownerId),
      members: room.memberIds.map(summary),
      playback_mode: room.mode as WatchRoomType["playback_mode"],
      is_owner: room.ownerId === viewerId,
      created_at: room.createdAt,
    };
  };

  const detail = (room: Room, viewerId: number): WatchRoomDetailType => ({
    ...listItem(room, viewerId),
    audio_track: room.audioTrack,
    subtitle_track: room.subtitleTrack,
  });

  /** Closes the given users' sockets in a room, telling each it lost access. */
  const revoke = (roomId: number, userIds: number[]) => {
    for (const socket of [...sockets]) {
      if (socket.roomId === roomId && userIds.includes(socket.userId)) {
        send(socket, { type: WATCH_ROOM_EVENT_TYPES.ROOM_ACCESS_REVOKED, room_id: roomId });
        socket.ws.close();
      }
    }
  };

  const deleteRoom = (roomId: number) => {
    rooms.delete(roomId);
    for (const socket of [...sockets]) {
      if (socket.roomId === roomId) {
        send(socket, { type: WATCH_ROOM_EVENT_TYPES.ROOM_DELETED, room_id: roomId });
        socket.ws.close();
      }
    }
  };

  const announceMembers = (room: Room) => {
    broadcast(room.id, {
      type: WATCH_ROOM_EVENT_TYPES.MEMBERS_CHANGED,
      room_id: room.id,
      members: room.memberIds.map(summary),
    });
  };

  const invitedIds = (body: Record<string, unknown>, ownerId: number) => {
    const raw = body.invited_user_ids;
    if (!Array.isArray(raw)) return null;
    const ids = [...new Set(raw.filter((id): id is number => typeof id === "number"))].filter(
      id => id !== ownerId,
    );
    return ids.every(id => deps.findUser(id)) ? ids : null;
  };

  async function handleRoute(
    request: IncomingMessage,
    response: ServerResponse,
    url: URL,
    userId: number,
  ): Promise<boolean> {
    const method = request.method ?? "GET";

    if (url.pathname === "/api/users" && method === "GET") {
      deps.sendSuccess(response, {
        users: deps
          .listUsers()
          .filter(user => user.id !== userId)
          .map(({ id, name, email, avatar }) => ({ id, name, email, avatar })),
      });
      return true;
    }

    if (url.pathname === "/api/watch-rooms" && method === "GET") {
      deps.sendSuccess(response, {
        rooms: [...rooms.values()]
          .filter(room => room.memberIds.includes(userId))
          .sort((a, b) => b.id - a.id)
          .map(room => listItem(room, userId)),
      });
      return true;
    }

    if (url.pathname === "/api/watch-rooms" && method === "POST") {
      const body = (await deps.readJSONBody(request)) as Partial<CreateWatchRoomRequestType> &
        Record<string, unknown>;
      if (typeof body.movie_id !== "number" || !deps.findMovie(body.movie_id)) {
        deps.sendFailure(response, 400, "movie not found");
        return true;
      }
      const invited = invitedIds({ invited_user_ids: body.invited_user_ids ?? [] }, userId);
      if (!invited) {
        deps.sendFailure(response, 400, "one or more invited users do not exist");
        return true;
      }
      const room: Room = {
        id: nextRoomId++,
        ownerId: userId,
        movieId: body.movie_id,
        mode: typeof body.mode === "string" ? body.mode : "direct",
        audioTrack: body.audio_track ?? 0,
        subtitleTrack: body.subtitle_track ?? null,
        memberIds: [userId, ...invited],
        createdAt: new Date().toISOString(),
        playback: { paused: true, positionSec: 0, updatedAt: Date.now() },
      };
      rooms.set(room.id, room);
      deps.sendSuccess(response, { room_id: room.id }, 201);
      return true;
    }

    const match = url.pathname.match(/^\/api\/watch-rooms\/(\d+)(\/[a-z]+)?$/);
    if (!match) return false;
    const roomId = Number(match[1]);
    const action = match[2] ?? "";
    const room = rooms.get(roomId);
    const member = room?.memberIds.includes(userId) ?? false;

    if (action === "" && method === "GET") {
      // An unknown room reads as a room the caller may not see.
      if (!room || !member) {
        deps.sendFailure(response, 403, ACCESS_DENIED);
        return true;
      }
      deps.sendSuccess(response, { room: detail(room, userId) });
      return true;
    }

    if (action === "/stream" && (method === "GET" || method === "HEAD")) {
      if (!room || !member) {
        deps.sendFailure(response, 403, ACCESS_DENIED);
        return true;
      }
      response.writeHead(200, { "Content-Type": "video/mp4", "Content-Length": "0" });
      response.end();
      return true;
    }

    if (!room) {
      deps.sendFailure(response, 404, ROOM_NOT_FOUND);
      return true;
    }

    if (action === "/join" && method === "POST") {
      if (!member) {
        deps.sendFailure(response, 403, ACCESS_DENIED);
        return true;
      }
      deps.sendSuccess(response, { room_id: room.id, joined: true });
      return true;
    }

    if (action === "/leave" && method === "POST") {
      if (room.ownerId === userId) {
        deps.sendFailure(response, 409, "the room owner cannot leave; close the room instead");
        return true;
      }
      if (!member) {
        deps.sendFailure(response, 403, ACCESS_DENIED);
        return true;
      }
      room.memberIds = room.memberIds.filter(id => id !== userId);
      revoke(room.id, [userId]);
      announceMembers(room);
      deps.sendSuccess(response, { left: true });
      return true;
    }

    if (action === "/members" && method === "PUT") {
      if (room.ownerId !== userId) {
        deps.sendFailure(response, 403, "only the room owner can change who is invited");
        return true;
      }
      const body = await deps.readJSONBody(request);
      if (!Array.isArray(body.invited_user_ids)) {
        deps.sendFailure(response, 400, "invited_user_ids is required");
        return true;
      }
      const invited = invitedIds(body, userId);
      if (!invited) {
        deps.sendFailure(response, 400, "one or more invited users do not exist");
        return true;
      }
      const removed = room.memberIds.filter(id => id !== userId && !invited.includes(id));
      room.memberIds = [
        userId,
        ...room.memberIds.filter(id => id !== userId && invited.includes(id)),
        ...invited.filter(id => !room.memberIds.includes(id)),
      ];
      revoke(room.id, removed);
      announceMembers(room);
      deps.sendSuccess(response, { room: detail(room, userId) });
      return true;
    }

    if (action === "" && method === "DELETE") {
      if (room.ownerId !== userId) {
        deps.sendFailure(response, 403, "only the room owner can delete this room");
        return true;
      }
      deleteRoom(room.id);
      deps.sendSuccess(response, { deleted: true });
      return true;
    }

    return false;
  }

  /**
   * Test controls, outside the contract: `drop` closes every socket in a room
   * the way a network blip does, so a spec can watch the page reconnect;
   * `stall` leaves the room's next socket upgrade unanswered, the way a proxy
   * that accepted the connection and then hung would.
   */
  function handleTestRoute(response: ServerResponse, url: URL): boolean {
    const match = url.pathname.match(/^\/api\/e2e\/watch-rooms\/(\d+)\/(drop|stall)$/);
    if (!match) return false;
    const roomId = Number(match[1]);
    if (match[2] === "stall") {
      stallNextUpgrade.add(roomId);
      deps.sendSuccess(response, { stalled: true });
      return true;
    }
    for (const socket of [...sockets]) {
      if (socket.roomId === roomId) socket.ws.terminate();
    }
    deps.sendSuccess(response, { dropped: true });
    return true;
  }

  function onSocketMessage(socket: RoomSocket, raw: string) {
    let event: { type?: unknown; position_sec?: unknown };
    try {
      event = JSON.parse(raw) as typeof event;
    } catch {
      return;
    }
    const room = rooms.get(socket.roomId);
    if (!room) return;

    switch (event.type) {
      case WATCH_ROOM_CLIENT_EVENT_TYPES.PING:
        send(socket, { type: WATCH_ROOM_EVENT_TYPES.PONG, room_id: room.id });
        return;
      case "join":
        send(socket, {
          type: WATCH_ROOM_EVENT_TYPES.ROOM_SNAPSHOT,
          room_id: room.id,
          playback: playbackSnapshot(room),
          connected_user_ids: connectedIds(room.id),
        });
        return;
      case WATCH_ROOM_CLIENT_EVENT_TYPES.PLAY:
      case WATCH_ROOM_CLIENT_EVENT_TYPES.PAUSE:
      case WATCH_ROOM_CLIENT_EVENT_TYPES.SEEK: {
        const now = Date.now();
        const position =
          typeof event.position_sec === "number" && event.position_sec >= 0
            ? event.position_sec
            : positionAt(room.playback, now);
        const paused =
          event.type === WATCH_ROOM_CLIENT_EVENT_TYPES.PLAY
            ? false
            : event.type === WATCH_ROOM_CLIENT_EVENT_TYPES.PAUSE
              ? true
              : room.playback.paused;
        room.playback = { paused, positionSec: position, updatedAt: now };
        // Everyone, the sender included, as the Go hub does.
        broadcast(room.id, {
          type: WATCH_ROOM_EVENT_TYPES.PLAYBACK_CHANGED,
          room_id: room.id,
          playback: playbackSnapshot(room),
        });
        return;
      }
      default:
        return;
    }
  }

  function onSocketClose(socket: RoomSocket) {
    if (!sockets.delete(socket)) return;
    const room = rooms.get(socket.roomId);
    const remaining = connectedIds(socket.roomId);
    if (!room) return;
    if (remaining.length === 0) {
      // An emptied room forgets where it was, as the server's does.
      room.playback = { paused: true, positionSec: 0, updatedAt: Date.now() };
      return;
    }
    if (!remaining.includes(socket.userId)) {
      broadcast(room.id, {
        type: WATCH_ROOM_EVENT_TYPES.MEMBER_LEFT,
        room_id: room.id,
        member: summary(socket.userId),
        connected_user_ids: remaining,
      });
    }
  }

  function refuseUpgrade(socket: Duplex, status: number, message: string) {
    const body = JSON.stringify({ error: true, message });
    socket.end(
      `HTTP/1.1 ${status} ${status === 401 ? "Unauthorized" : "Forbidden"}\r\n` +
        "Content-Type: application/json\r\n" +
        `Content-Length: ${Buffer.byteLength(body)}\r\n\r\n${body}`,
    );
  }

  function attach(server: Server) {
    server.on("upgrade", (request: IncomingMessage, rawSocket: Duplex, head: Buffer) => {
      const url = new URL(request.url ?? "/", "http://127.0.0.1");
      const match = url.pathname.match(/^\/api\/watch-rooms\/(\d+)\/ws$/);
      if (!match) {
        rawSocket.destroy();
        return;
      }
      const userId = deps.currentUserId(request);
      if (userId === null) {
        refuseUpgrade(rawSocket, 401, "Unauthorized");
        return;
      }
      const room = rooms.get(Number(match[1]));
      if (!room || !room.memberIds.includes(userId)) {
        refuseUpgrade(rawSocket, 403, ACCESS_DENIED);
        return;
      }
      if (stallNextUpgrade.delete(room.id)) {
        stalledSockets.add(rawSocket);
        rawSocket.on("close", () => stalledSockets.delete(rawSocket));
        return;
      }

      wss.handleUpgrade(request, rawSocket, head, ws => {
        const firstConnection = !connectedIds(room.id).includes(userId);
        const socket: RoomSocket = { ws, roomId: room.id, userId };
        sockets.add(socket);
        send(socket, {
          type: WATCH_ROOM_EVENT_TYPES.ROOM_SNAPSHOT,
          room_id: room.id,
          playback: playbackSnapshot(room),
          connected_user_ids: connectedIds(room.id),
        });
        if (firstConnection) {
          broadcast(
            room.id,
            {
              type: WATCH_ROOM_EVENT_TYPES.MEMBER_JOINED,
              room_id: room.id,
              member: summary(userId),
              connected_user_ids: connectedIds(room.id),
            },
            socket,
          );
        }
        ws.on("message", data => {
          const raw = Array.isArray(data)
            ? Buffer.concat(data)
            : Buffer.isBuffer(data)
              ? data
              : Buffer.from(data);
          onSocketMessage(socket, raw.toString("utf8"));
        });
        ws.on("close", () => onSocketClose(socket));
      });
    });
  }

  /** Back to no rooms: drops every socket, then forgets the rooms. */
  function reset() {
    for (const socket of [...sockets]) socket.ws.terminate();
    sockets.clear();
    for (const rawSocket of [...stalledSockets]) rawSocket.destroy();
    stalledSockets.clear();
    stallNextUpgrade.clear();
    rooms.clear();
    nextRoomId = 1;
  }

  /**
   * A deleted account leaves every room, and the rooms it owned go. Its own
   * sockets close without a word, as the server closes a revoked
   * credential's; their reconnect is refused.
   */
  function forgetUser(userId: number) {
    for (const socket of [...sockets]) {
      if (socket.userId === userId) socket.ws.close();
    }
    for (const room of [...rooms.values()]) {
      if (room.ownerId === userId) {
        deleteRoom(room.id);
      } else if (room.memberIds.includes(userId)) {
        room.memberIds = room.memberIds.filter(id => id !== userId);
        announceMembers(room);
      }
    }
  }

  return { handleRoute, handleTestRoute, attach, forgetUser, reset };
}
