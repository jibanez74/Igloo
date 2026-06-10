import type { StreamModeId } from "./playback";

export type WatchRoomMemberType = {
  id: number;
  name: string;
  avatar: string | null;
};

export type WatchRoomInviteUserType = {
  id: number;
  name: string;
  email: string;
  avatar: string | null;
};

export type WatchRoomType = {
  id: number;
  movie_id: number;
  movie_title: string;
  movie_poster: string | null;
  owner: WatchRoomMemberType;
  members: WatchRoomMemberType[];
  playback_mode: StreamModeId;
  is_owner: boolean;
  created_at: string;
};

export type WatchRoomDetailType = {
  id: number;
  movie_id: number;
  movie_title: string;
  movie_poster: string | null;
  owner: WatchRoomMemberType;
  members: WatchRoomMemberType[];
  playback_mode: StreamModeId;
  audio_track: number;
  subtitle_track: number | null;
  is_owner: boolean;
  created_at: string;
};

export type WatchRoomInviteUsersResponseType = {
  users: WatchRoomInviteUserType[];
};

export type CreateWatchRoomRequestType = {
  movie_id: number;
  mode: StreamModeId;
  audio_track: number;
  subtitle_track: number | null;
  invited_user_ids: number[];
};

export type CreateWatchRoomResponseType = {
  room_id: number;
};

export type WatchRoomResponseType = {
  room: WatchRoomDetailType;
};

export type JoinWatchRoomResponseType = {
  room_id: number;
  joined: boolean;
};

export type WatchRoomPlaybackStateType = {
  paused: boolean;
  position_sec: number;
  updated_at: string;
};

export type WatchRoomServerEventType = {
  type:
    | "room_snapshot"
    | "playback_changed"
    | "member_joined"
    | "member_left"
    | "room_deleted"
    | "pong";
  room_id: number;
  playback?: WatchRoomPlaybackStateType;
  member?: WatchRoomMemberType;
  connected_user_ids?: number[];
};
