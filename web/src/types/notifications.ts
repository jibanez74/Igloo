import type { components } from "./openapi.gen";

export type NotificationType = components["schemas"]["Notification"];
export type NotificationTitle = NotificationType["title"];
export type CreateNotificationRequest = components["schemas"]["CreateNotificationRequest"];
export type CreateNotificationResponseType = components["schemas"]["CreateNotificationEnvelope"]["data"];
export type NotificationListItemType = components["schemas"]["NotificationListItem"];
export type NotificationsListResponseType = components["schemas"]["NotificationsListEnvelope"]["data"];
export type UnreadNotificationCountResponseType = components["schemas"]["UnreadNotificationCountEnvelope"]["data"];
