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
  NOTIFICATION_TITLES,
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
  NotificationTitle,
  NotificationsListResponseType,
  UnreadNotificationCountResponseType,
} from "@/types";

const NOTIFICATION_TITLE_LABELS: Partial<Record<string, string>> = {
  [NOTIFICATION_TITLES.MOVIE_REQUEST]: "Movie request",
  [NOTIFICATION_TITLES.ALBUM_REQUEST]: "Album request",
  [NOTIFICATION_TITLES.TRACK_REQUEST]: "Track request",
  [NOTIFICATION_TITLES.OTHER]: "Notification",
} satisfies Record<NotificationTitle, string>;

function notificationTitleLabel(title: string): string {
  return NOTIFICATION_TITLE_LABELS[title] ?? "Notification";
}

const RELATIVE_TIME_FORMAT = new Intl.RelativeTimeFormat(undefined, {
  numeric: "auto",
});

// SQLite timestamps come back as "YYYY-MM-DD HH:MM:SS" in UTC; normalize to an
// ISO string before parsing so the relative time is computed correctly.
function formatRelativeTime(timestamp: string): string {
  const parsed = new Date(`${timestamp.replace(" ", "T")}Z`).getTime();
  if (Number.isNaN(parsed)) return "";

  const diffSec = Math.round((parsed - Date.now()) / 1000);
  const abs = Math.abs(diffSec);
  const rtf = RELATIVE_TIME_FORMAT;

  if (abs < 60) return rtf.format(diffSec, "second");
  if (abs < 3600) return rtf.format(Math.round(diffSec / 60), "minute");
  if (abs < 86400) return rtf.format(Math.round(diffSec / 3600), "hour");
  if (abs < 604800) return rtf.format(Math.round(diffSec / 86400), "day");
  return rtf.format(Math.round(diffSec / 604800), "week");
}

export default function NotificationBell() {
  const [open, setOpen] = useState(false);
  const queryClient = useQueryClient();

  // Both queries return the `{ error, data }` envelope as query data, so narrow
  // on `error === false` before reading the success shape (like every other
  // query consumer). An error envelope surfaces as `showRefreshError`, not a
  // thrown query error.
  const { data: countEnvelope, isError: countIsError } = useQuery(
    unreadNotificationCountQueryOpts(),
  );
  const freshCountData =
    countEnvelope?.error === false ? countEnvelope.data : undefined;

  // Only fetch the full list while the panel is open.
  const {
    data: listEnvelope,
    isError: listIsError,
    isLoading: listIsLoading,
  } = useQuery({
    ...notificationsQueryOpts(),
    enabled: open,
  });
  const freshListData =
    listEnvelope?.error === false ? listEnvelope.data : undefined;

  // react-query stores an error envelope as (successful) data, dropping the last
  // good payload. Retain it so a failed refresh keeps showing the previous list
  // and badge instead of flashing empty. Adjusting state during render is the
  // supported way to remember a value across renders here (React re-renders
  // immediately without committing the intermediate paint).
  const [lastCountData, setLastCountData] =
    useState<UnreadNotificationCountResponseType>();
  const [lastListData, setLastListData] =
    useState<NotificationsListResponseType>();
  if (freshCountData && freshCountData !== lastCountData) {
    setLastCountData(freshCountData);
  }
  if (freshListData && freshListData !== lastListData) {
    setLastListData(freshListData);
  }

  const listData = freshListData ?? lastListData;
  const countUnreadCount = (freshCountData ?? lastCountData)?.unread_count ?? 0;
  const notifications = listData?.notifications ?? [];
  const listUnreadCount = open ? listData?.unread_count : undefined;
  const unreadCount = listUnreadCount ?? countUnreadCount;
  const showRefreshError = Boolean(
    countIsError ||
      countEnvelope?.error ||
      (open && (listIsError || listEnvelope?.error)),
  );
  const showInitialLoading = listIsLoading && !listEnvelope;
  const showEmptyState = !!listData && notifications.length === 0;

  useEffect(() => {
    if (!freshListData) {
      return;
    }

    queryClient.setQueryData<
      ApiResponseType<UnreadNotificationCountResponseType>
    >([NOTIFICATIONS_UNREAD_COUNT_KEY], {
      error: false,
      data: { unread_count: freshListData.unread_count },
    });
  }, [freshListData, queryClient]);

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
          className="relative text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          <Bell aria-hidden="true" />
          {unreadCount > 0 && (
            <span
              aria-hidden="true"
              className="absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-primary px-1 text-[10px] font-semibold text-primary-foreground"
            >
              {unreadCount > 99 ? "99+" : unreadCount}
            </span>
          )}
        </Button>
      </PopoverTrigger>

      <PopoverContent
        align="end"
        className="w-80 border-border bg-card p-0 text-foreground"
      >
        <div className="flex items-center justify-between border-b border-border px-4 py-3">
          <h2 className="text-sm font-semibold text-foreground">Notifications</h2>
          {unreadCount > 0 && (
            <button
              type="button"
              onClick={() => markAllMutation.mutate()}
              disabled={markAllMutation.isPending}
              className="text-xs font-medium text-primary hover:text-primary disabled:opacity-50"
            >
              Mark all read
            </button>
          )}
        </div>

        <div className="max-h-96 overflow-y-auto">
          {showRefreshError && (
            <p
              role="alert"
              className="border-b border-border px-4 py-3 text-sm text-primary"
            >
              Unable to refresh notifications.
            </p>
          )}
          {showInitialLoading ? (
            <div className="flex items-center justify-center py-10">
              <Spinner aria-label="Loading notifications" />
            </div>
          ) : showEmptyState ? (
            <p className="px-4 py-10 text-center text-sm text-muted-foreground">
              You&apos;re all caught up.
            </p>
          ) : notifications.length > 0 ? (
            <ul className="divide-y divide-border">
              {notifications.map((notification) => {
                const titleLabel = notificationTitleLabel(notification.title);
                const readStateLabel = notification.is_read ? "Read" : "Unread";
                const relativeTimeLabel = formatRelativeTime(
                  notification.created_at,
                );
                const messageLabel = notification.message
                  .replace(/\s+/g, " ")
                  .trim();
                const itemLabelParts = [
                  `${readStateLabel} notification`,
                  titleLabel,
                ];
                const dismissLabelParts = [
                  `Dismiss ${readStateLabel.toLowerCase()} notification`,
                  titleLabel,
                ];

                if (notification.created_by_name) {
                  itemLabelParts.push(`from ${notification.created_by_name}`);
                  dismissLabelParts.push(`from ${notification.created_by_name}`);
                }

                if (messageLabel) {
                  itemLabelParts.push(messageLabel);
                  dismissLabelParts.push(messageLabel);
                }

                if (relativeTimeLabel) {
                  itemLabelParts.push(relativeTimeLabel);
                }

                return (
                  <li
                    key={notification.id}
                    className={cn(
                      "flex items-start gap-2 px-4 py-3",
                      notification.is_read
                        ? "bg-transparent"
                        : "bg-muted/40",
                    )}
                  >
                    <button
                      type="button"
                      onClick={() => handleItemClick(notification)}
                      aria-label={itemLabelParts.join(", ")}
                      className="min-w-0 flex-1 text-left"
                    >
                      <div className="flex items-center gap-2">
                        {!notification.is_read && (
                          <span
                            aria-hidden="true"
                            className="size-2 shrink-0 rounded-full bg-primary"
                          />
                        )}
                        <span className="truncate text-sm font-medium text-foreground">
                          {titleLabel}
                        </span>
                        <span className="ml-auto shrink-0 text-xs text-muted-foreground">
                          {relativeTimeLabel}
                        </span>
                      </div>
                      <p className="mt-1 text-xs whitespace-pre-line text-muted-foreground">
                        {notification.message}
                      </p>
                      {notification.created_by_name && (
                        <p className="mt-1 text-xs text-muted-foreground">
                          From {notification.created_by_name}
                        </p>
                      )}
                    </button>

                    <button
                      type="button"
                      onClick={() => deleteMutation.mutate(notification.id)}
                      disabled={deleteMutation.isPending}
                      aria-label={dismissLabelParts.join(", ")}
                      className="shrink-0 rounded-md p-1 text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-50"
                    >
                      <X aria-hidden="true" className="size-4" />
                    </button>
                  </li>
                );
              })}
            </ul>
          ) : null}
        </div>
      </PopoverContent>
    </Popover>
  );
}
