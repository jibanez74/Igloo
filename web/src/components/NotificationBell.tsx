import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Bell, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Spinner } from "@/components/ui/spinner";
import {
  deleteNotification,
  markAllNotificationsRead,
  markNotificationRead,
} from "@/lib/api";
import {
  NOTIFICATIONS_KEY,
  NOTIFICATIONS_UNREAD_COUNT_KEY,
} from "@/lib/constants";
import {
  notificationsQueryOpts,
  unreadNotificationCountQueryOpts,
} from "@/lib/query-opts";
import { showActionFailed } from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import type {
  ApiResponseType,
  NotificationListItemType,
  UnreadNotificationCountResponseType,
} from "@/types";

const NOTIFICATION_TITLE_LABELS: Record<string, string> = {
  movie_request: "Movie request",
  album_request: "Album request",
  track_request: "Track request",
  other: "Notification",
};

function notificationTitleLabel(title: string): string {
  return NOTIFICATION_TITLE_LABELS[title] ?? "Notification";
}

// SQLite timestamps come back as "YYYY-MM-DD HH:MM:SS" in UTC; normalize to an
// ISO string before parsing so the relative time is computed correctly.
function formatRelativeTime(timestamp: string): string {
  const parsed = new Date(`${timestamp.replace(" ", "T")}Z`).getTime();
  if (Number.isNaN(parsed)) return "";

  const diffSec = Math.round((parsed - Date.now()) / 1000);
  const abs = Math.abs(diffSec);
  const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" });

  if (abs < 60) return rtf.format(diffSec, "second");
  if (abs < 3600) return rtf.format(Math.round(diffSec / 60), "minute");
  if (abs < 86400) return rtf.format(Math.round(diffSec / 3600), "hour");
  if (abs < 604800) return rtf.format(Math.round(diffSec / 86400), "day");
  return rtf.format(Math.round(diffSec / 604800), "week");
}

export default function NotificationBell() {
  const [open, setOpen] = useState(false);
  const queryClient = useQueryClient();

  const countQuery = useQuery(unreadNotificationCountQueryOpts());
  const countUnreadCount =
    countQuery.data?.error === false ? countQuery.data.data.unread_count : 0;

  // Only fetch the full list while the panel is open.
  const listQuery = useQuery({
    ...notificationsQueryOpts(),
    enabled: open,
  });
  const notifications =
    listQuery.data?.error === false ? listQuery.data.data.notifications : [];
  const listUnreadCount =
    open && listQuery.data?.error === false
      ? listQuery.data.data.unread_count
      : undefined;
  const unreadCount = listUnreadCount ?? countUnreadCount;

  useEffect(() => {
    if (listQuery.data?.error !== false) {
      return;
    }

    queryClient.setQueryData<
      ApiResponseType<UnreadNotificationCountResponseType>
    >([NOTIFICATIONS_UNREAD_COUNT_KEY], {
      error: false,
      data: { unread_count: listQuery.data.data.unread_count },
    });
  }, [listQuery.data, queryClient]);

  function invalidateNotifications() {
    void queryClient.invalidateQueries({ queryKey: [NOTIFICATIONS_KEY] });
    void queryClient.invalidateQueries({
      queryKey: [NOTIFICATIONS_UNREAD_COUNT_KEY],
    });
  }

  const markReadMutation = useMutation({
    mutationFn: (id: number) => markNotificationRead(id),
    onSuccess: (res) => {
      if (res.error) {
        showActionFailed("update notification", res.message);
        return;
      }
      invalidateNotifications();
    },
    onError: () => showActionFailed("update notification"),
  });

  const markAllMutation = useMutation({
    mutationFn: () => markAllNotificationsRead(),
    onSuccess: (res) => {
      if (res.error) {
        showActionFailed("mark all as read", res.message);
        return;
      }
      invalidateNotifications();
    },
    onError: () => showActionFailed("mark all as read"),
  });

  const deleteMutation = useMutation({
    mutationFn: (id: number) => deleteNotification(id),
    onSuccess: (res) => {
      if (res.error) {
        showActionFailed("dismiss notification", res.message);
        return;
      }
      invalidateNotifications();
    },
    onError: () => showActionFailed("dismiss notification"),
  });

  function handleItemClick(notification: NotificationListItemType) {
    if (!notification.is_read) {
      markReadMutation.mutate(notification.id);
    }
  }

  const bellLabel =
    unreadCount > 0 ? `Notifications, ${unreadCount} unread` : "Notifications";

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          variant="ghost"
          size="icon"
          aria-label={bellLabel}
          className="relative text-slate-300 hover:bg-slate-800 hover:text-white"
        >
          <Bell aria-hidden="true" />
          {unreadCount > 0 && (
            <span
              aria-hidden="true"
              className="absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-amber-500 px-1 text-[10px] font-semibold text-slate-950"
            >
              {unreadCount > 99 ? "99+" : unreadCount}
            </span>
          )}
        </Button>
      </PopoverTrigger>

      <PopoverContent
        align="end"
        className="w-80 border-slate-700 bg-slate-900 p-0 text-slate-100"
      >
        <div className="flex items-center justify-between border-b border-slate-800 px-4 py-3">
          <h2 className="text-sm font-semibold text-white">Notifications</h2>
          {unreadCount > 0 && (
            <button
              type="button"
              onClick={() => markAllMutation.mutate()}
              disabled={markAllMutation.isPending}
              className="text-xs font-medium text-amber-400 hover:text-amber-300 disabled:opacity-50"
            >
              Mark all read
            </button>
          )}
        </div>

        <div className="max-h-96 overflow-y-auto">
          {listQuery.isLoading ? (
            <div className="flex items-center justify-center py-10">
              <Spinner aria-label="Loading notifications" />
            </div>
          ) : notifications.length === 0 ? (
            <p className="px-4 py-10 text-center text-sm text-slate-400">
              You&apos;re all caught up.
            </p>
          ) : (
            <ul className="divide-y divide-slate-800">
              {notifications.map((notification) => (
                <li
                  key={notification.id}
                  className={cn(
                    "flex items-start gap-2 px-4 py-3",
                    notification.is_read ? "bg-transparent" : "bg-slate-800/40",
                  )}
                >
                  <button
                    type="button"
                    onClick={() => handleItemClick(notification)}
                    className="min-w-0 flex-1 text-left"
                  >
                    <div className="flex items-center gap-2">
                      {!notification.is_read && (
                        <span
                          aria-hidden="true"
                          className="size-2 shrink-0 rounded-full bg-amber-400"
                        />
                      )}
                      <span className="truncate text-sm font-medium text-white">
                        {notificationTitleLabel(notification.title)}
                      </span>
                      <span className="ml-auto shrink-0 text-xs text-slate-500">
                        {formatRelativeTime(notification.created_at)}
                      </span>
                    </div>
                    <p className="mt-1 text-xs whitespace-pre-line text-slate-300">
                      {notification.message}
                    </p>
                    {notification.created_by_name && (
                      <p className="mt-1 text-xs text-slate-500">
                        From {notification.created_by_name}
                      </p>
                    )}
                  </button>

                  <button
                    type="button"
                    onClick={() => deleteMutation.mutate(notification.id)}
                    disabled={deleteMutation.isPending}
                    aria-label="Dismiss notification"
                    className="shrink-0 rounded-md p-1 text-slate-500 hover:bg-slate-800 hover:text-white disabled:opacity-50"
                  >
                    <X aria-hidden="true" className="size-4" />
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}
