import type { components } from "./openapi.gen";

type Schema = components["schemas"];

export type WatchRoomMemberType = Schema["WatchRoomMember"];
export type WatchRoomInviteUserType = Schema["InviteUser"];
export type WatchRoomType = Schema["WatchRoomListItem"];
export type WatchRoomDetailType = Schema["WatchRoomDetail"];
export type WatchRoomInviteUsersResponseType = Schema["InviteUsersData"];
export type CreateWatchRoomRequestType = Schema["CreateWatchRoomRequest"];
export type CreateWatchRoomResponseType = Schema["CreateWatchRoomData"];
export type WatchRoomResponseType = Schema["WatchRoomData"];
export type JoinWatchRoomResponseType = Schema["JoinWatchRoomData"];
export type WatchRoomPlaybackStateType = Schema["WatchRoomPlaybackState"];
export type WatchRoomServerEventType = Schema["WatchRoomServerEvent"];
