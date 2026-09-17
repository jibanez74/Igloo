import type { components } from "./openapi.gen";

export type NotificationTitle = NotificationListItemType["title"];
export type CreateNotificationRequest = components["schemas"]["CreateNotificationRequest"];
export type NotificationListItemType = components["schemas"]["NotificationListItem"];
export type NotificationsListResponseType = components["schemas"]["NotificationsListData"];
export type UnreadNotificationCountResponseType = components["schemas"]["UnreadNotificationCountData"];
export type CreateNotificationResponseType = components["schemas"]["CreateNotificationEnvelope"];
