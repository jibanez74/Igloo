import type { components } from "./openapi.gen";

type Schema = components["schemas"];

export type WatchRoomMemberType = Schema["WatchRoomMember"];
export type WatchRoomInviteUserType = Schema["InviteUser"];
export type WatchRoomType = Schema["WatchRoomListItem"];
export type WatchRoomDetailType = Schema["WatchRoomDetail"];
export type WatchRoomInviteUsersResponseType = Schema["InviteUsersEnvelope"]["data"];
export type CreateWatchRoomRequestType = Schema["CreateWatchRoomRequest"];
export type CreateWatchRoomResponseType = Schema["CreateWatchRoomEnvelope"]["data"];
export type WatchRoomResponseType = Schema["WatchRoomEnvelope"]["data"];
export type JoinWatchRoomResponseType = Schema["JoinWatchRoomEnvelope"]["data"];
export type WatchRoomPlaybackStateType = Schema["WatchRoomPlaybackState"];
export type WatchRoomServerEventType = Schema["WatchRoomServerEvent"];
