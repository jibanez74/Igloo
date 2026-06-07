export type NotificationType = {
  id: number;
  created_by_user_id: number;
  user_id: number | null;
  title: string;
  message: string;
  is_admin: boolean;
  created_at: string;
  updated_at: string;
};

export type CreateNotificationRequest = {
  title: string;
  message: string;
  isAdmin: boolean;
};

export type CreateNotificationResponseType = {
  notification: NotificationType;
};
