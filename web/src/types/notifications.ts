import type { NOTIFICATION_TITLES } from "@/lib/constants";

export type NotificationTitle =
  (typeof NOTIFICATION_TITLES)[keyof typeof NOTIFICATION_TITLES];

export type NotificationType = {
  id: number;
  created_by_user_id: number;
  user_id: number | null;
  title: NotificationTitle;
  message: string;
  is_admin: boolean;
  created_at: string;
  updated_at: string;
};

export type CreateNotificationRequest = {
  title: NotificationTitle;
  message: string;
  isAdmin: boolean;
};

export type CreateNotificationResponseType = {
  notification: NotificationType;
};

// A notification as returned by the list endpoint: the creator's display name
// and the viewer's read state are resolved server-side.
export type NotificationListItemType = {
  id: number;
  title: NotificationTitle;
  message: string;
  is_admin: boolean;
  is_read: boolean;
  created_by_name: string;
  user_id: number | null;
  created_at: string;
};

export type NotificationsListResponseType = {
  notifications: NotificationListItemType[];
  unread_count: number;
};

export type UnreadNotificationCountResponseType = {
  unread_count: number;
};
