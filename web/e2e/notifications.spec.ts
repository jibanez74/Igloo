import { expect, test } from "./e2e-test";
import type { APIRequestContext, Page } from "@playwright/test";

import type { NotificationTitle } from "../src/types";
import type { components } from "../src/types/openapi.gen";
import { getApiData } from "./e2e-api";
import { loginPageViaApi, loginViaApi } from "./e2e-auth";
import { readE2EEnv, requireMockApi } from "./e2e-env";
import { createUser, deleteUsersByEmailPrefix } from "./e2e-users";

type NotificationsList = components["schemas"]["NotificationsListData"];

// Readers file requests (movies, albums, tracks); every administrator sees
// them in the bell. The request dialogs belong to the library specs, so the
// reader here files over the API, as those dialogs do.

async function fileRequest(
  request: APIRequestContext,
  title: NotificationTitle,
  message: string,
) {
  const response = await request.post("/api/notifications", {
    data: { title, message },
    failOnStatusCode: false,
  });
  expect(response.status(), "POST /api/notifications").toBe(201);
}

function fetchNotifications(request: APIRequestContext) {
  return getApiData<NotificationsList>(request, "/api/notifications");
}

function escapeRegExp(text: string) {
  return text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function bellLabel(unread: number) {
  return unread > 0 ? `Notifications, ${unread} unread` : "Notifications";
}

/**
 * Signs the admin's page in and runs `run` with a disposable reader's request
 * context. Deleting the reader afterwards deletes their requests with them
 * (the server cascades), so a real instance keeps only its own.
 */
async function withRequester(
  page: Page,
  newContext: (baseURL: string) => Promise<APIRequestContext>,
  run: (requester: APIRequestContext, name: string) => Promise<void>,
) {
  const stamp = Date.now();
  const prefix = `playwright-notifications-${stamp}`;
  const reader = {
    name: `Playwright Requester ${stamp}`,
    email: `${prefix}@example.com`,
    password: `RequestPass${stamp}!`,
  };
  const admin = page.context().request;
  const requester = await newContext(readE2EEnv().baseURL);

  await loginPageViaApi(page);
  await createUser(admin, reader);

  try {
    await loginViaApi(requester, reader);
    await run(requester, reader.name);
  } finally {
    await requester.dispose();
    await deleteUsersByEmailPrefix(admin, prefix);
  }
}

test.describe("Notifications", () => {
  test("an administrator reads a reader's request from the bell and dismisses both", async ({
    page,
    playwright,
  }) => {
    await withRequester(
      page,
      baseURL => playwright.request.newContext({ baseURL }),
      async (requester, name) => {
        const stamp = Date.now();
        const movieMessage = `Please add Signal Fire (${stamp})`;
        const albumMessage = `Please add Night Tide (${stamp})`;
        // A real instance may hold requests of its own; count from them.
        const before = (await fetchNotifications(page.context().request))
          .unread_count;

        await fileRequest(requester, "movie_request", movieMessage);
        await fileRequest(requester, "album_request", albumMessage);

        await page.goto("/");
        const bell = page.getByRole("button", { name: /^Notifications/ });
        await expect(bell).toHaveAccessibleName(bellLabel(before + 2));

        await bell.click();
        // A row's name runs on with how long ago it was filed.
        const row = (state: "Unread" | "Read", kind: string, message: string) =>
          page.getByRole("button", {
            name: new RegExp(`^${escapeRegExp(`${state} notification, ${kind}, from ${name}, ${message}, `)}`),
          });
        const dismiss = (state: "unread" | "read", kind: string, message: string) =>
          page.getByRole("button", {
            name: `Dismiss ${state} notification, ${kind}, from ${name}, ${message}`,
            exact: true,
          });

        // Newest first.
        const rows = page.getByRole("button", {
          name: new RegExp(`^(Unread|Read) notification, .+, from ${escapeRegExp(name)}, `),
        });
        await expect(rows).toHaveCount(2);
        await expect(rows.first()).toHaveAccessibleName(
          new RegExp(`^Unread notification, Album request, from ${escapeRegExp(name)}, `),
        );

        // Opening one marks it read for this administrator.
        await row("Unread", "Movie request", movieMessage).click();
        await expect(row("Read", "Movie request", movieMessage)).toBeVisible();
        await expect(bell).toHaveAccessibleName(bellLabel(before + 1));

        // Dismissing clears a request, read or not.
        await dismiss("unread", "Album request", albumMessage).click();
        await expect(row("Unread", "Album request", albumMessage)).toHaveCount(0);
        await expect(bell).toHaveAccessibleName(bellLabel(before));
        await dismiss("read", "Movie request", movieMessage).click();
        await expect(rows).toHaveCount(0);

        const after = await fetchNotifications(page.context().request);
        expect(
          after.notifications.filter(item => item.message.includes(String(stamp))),
        ).toEqual([]);
        expect(after.unread_count).toBe(before);
      },
    );
  });

  test("Mark all read clears the badge, and it stays cleared after a reload", async ({
    page,
    playwright,
  }) => {
    // On a real instance it would also mark the instance's own requests read.
    requireMockApi();
    await withRequester(
      page,
      baseURL => playwright.request.newContext({ baseURL }),
      async requester => {
        await fileRequest(requester, "track_request", "Please add Low Light");
        await fileRequest(requester, "other", "The subtitles on Frost Harbor are off");

        await page.goto("/");
        const bell = page.getByRole("button", { name: /^Notifications/ });
        await expect(bell).toHaveAccessibleName(bellLabel(2));

        await bell.click();
        await page.getByRole("button", { name: "Mark all read" }).click();
        await expect(bell).toHaveAccessibleName(bellLabel(0));
        await expect(page.getByRole("button", { name: /^Unread notification/ })).toHaveCount(0);
        await expect(page.getByRole("button", { name: /^Read notification/ })).toHaveCount(2);
        await expect(page.getByRole("button", { name: "Mark all read" })).toHaveCount(0);

        await page.reload();
        await expect(page.getByRole("button", { name: "Notifications", exact: true })).toBeVisible();
      },
    );
  });
});
