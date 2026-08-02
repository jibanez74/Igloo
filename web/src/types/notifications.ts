import type { components } from "./openapi.gen";

export type NotificationType = components["schemas"]["Notification"];
export type NotificationTitle = NotificationType["title"];
export type CreateNotificationRequest = components["schemas"]["CreateNotificationRequest"];
export type CreateNotificationResponseType = components["schemas"]["CreateNotificationData"];
export type NotificationListItemType = components["schemas"]["NotificationListItem"];
export type NotificationsListResponseType = components["schemas"]["NotificationsListData"];
export type UnreadNotificationCountResponseType = components["schemas"]["UnreadNotificationCountData"];
