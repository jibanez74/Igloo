import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { PropsWithChildren } from "react";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import NotificationBell from "@/components/NotificationBell";
import type {
  ApiResponseType,
  NotificationListItemType,
  NotificationsListResponseType,
  UnreadNotificationCountResponseType,
} from "@/types";

const apiMocks = vi.hoisted(() => ({
  getNotifications: vi.fn(),
  getUnreadNotificationCount: vi.fn(),
  markNotificationRead: vi.fn(),
  markAllNotificationsRead: vi.fn(),
  deleteNotification: vi.fn(),
}));
const toastMocks = vi.hoisted(() => ({
  showActionFailed: vi.fn(),
}));

vi.mock("@/lib/api", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/api")>("@/lib/api");

  return {
    ...actual,
    getNotifications: (...args: unknown[]) => apiMocks.getNotifications(...args),
    getUnreadNotificationCount: (...args: unknown[]) =>
      apiMocks.getUnreadNotificationCount(...args),
    markNotificationRead: (...args: unknown[]) =>
      apiMocks.markNotificationRead(...args),
    markAllNotificationsRead: (...args: unknown[]) =>
      apiMocks.markAllNotificationsRead(...args),
    deleteNotification: (...args: unknown[]) =>
      apiMocks.deleteNotification(...args),
  };
});

vi.mock("@/lib/toast-helpers", () => ({
  showActionFailed: toastMocks.showActionFailed,
}));

// Radix Popover relies on pointer capture APIs that jsdom does not implement.
beforeAll(() => {
  Element.prototype.hasPointerCapture = vi.fn();
  Element.prototype.setPointerCapture = vi.fn();
  Element.prototype.releasePointerCapture = vi.fn();
  Element.prototype.scrollIntoView = vi.fn();
});

function success<T extends Record<string, unknown>>(data: T): ApiResponseType<T> {
  return {
    error: false,
    data,
  };
}

function notification(
  overrides: Partial<NotificationListItemType> = {},
): NotificationListItemType {
  return {
    id: 1,
    title: "movie_request",
    message: "Requester wants Dune",
    is_admin: true,
    is_read: false,
    created_by_name: "Music Fan",
    user_id: null,
    created_at: "2026-01-01 00:00:00",
    ...overrides,
  };
}

function listResponse(
  notifications: NotificationListItemType[],
): ApiResponseType<NotificationsListResponseType> {
  return success({
    notifications,
    unread_count: notifications.filter(item => !item.is_read).length,
  });
}

function countResponse(
  unread: number,
): ApiResponseType<UnreadNotificationCountResponseType> {
  return success({ unread_count: unread });
}

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      mutations: { retry: false },
      queries: { retry: false },
    },
  });
}

function renderBell() {
  const queryClient = createQueryClient();

  function Wrapper({ children }: PropsWithChildren) {
    return (
      <QueryClientProvider client={queryClient}>
        {children}
      </QueryClientProvider>
    );
  }

  render(<NotificationBell />, { wrapper: Wrapper });
}

beforeEach(() => {
  apiMocks.getNotifications.mockResolvedValue(listResponse([notification()]));
  apiMocks.getUnreadNotificationCount.mockResolvedValue(countResponse(1));
  apiMocks.markNotificationRead.mockResolvedValue(success({}));
  apiMocks.markAllNotificationsRead.mockResolvedValue(success({}));
  apiMocks.deleteNotification.mockResolvedValue(success({}));
});

describe("NotificationBell", () => {
  it("shows the unread count in the badge and the accessible label", async () => {
    apiMocks.getUnreadNotificationCount.mockResolvedValue(countResponse(3));

    renderBell();

    const trigger = await screen.findByRole("button", {
      name: "Notifications, 3 unread",
    });
    expect(trigger).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
  });

  it("opens the panel, lists notifications, and marks an unread one read on click", async () => {
    const user = userEvent.setup();
    renderBell();

    await user.click(
      await screen.findByRole("button", { name: "Notifications, 1 unread" }),
    );

    expect(await screen.findByText("Requester wants Dune")).toBeVisible();
    expect(screen.getByText("Movie request")).toBeVisible();

    await user.click(screen.getByText("Requester wants Dune"));

    await waitFor(() => {
      expect(apiMocks.markNotificationRead).toHaveBeenCalledWith(1);
    });
  });

  it("marks all notifications read", async () => {
    const user = userEvent.setup();
    renderBell();

    await user.click(
      await screen.findByRole("button", { name: "Notifications, 1 unread" }),
    );

    await user.click(
      await screen.findByRole("button", { name: "Mark all read" }),
    );

    await waitFor(() => {
      expect(apiMocks.markAllNotificationsRead).toHaveBeenCalledTimes(1);
    });
  });

  it("dismisses a notification", async () => {
    const user = userEvent.setup();
    renderBell();

    await user.click(
      await screen.findByRole("button", { name: "Notifications, 1 unread" }),
    );

    await user.click(
      await screen.findByRole("button", { name: "Dismiss notification" }),
    );

    await waitFor(() => {
      expect(apiMocks.deleteNotification).toHaveBeenCalledWith(1);
    });
  });

  it("shows the empty state when there are no notifications", async () => {
    apiMocks.getUnreadNotificationCount.mockResolvedValue(countResponse(0));
    apiMocks.getNotifications.mockResolvedValue(listResponse([]));

    const user = userEvent.setup();
    renderBell();

    const trigger = await screen.findByRole("button", { name: "Notifications" });
    await user.click(trigger);

    expect(await screen.findByText("You're all caught up.")).toBeVisible();
  });
});
