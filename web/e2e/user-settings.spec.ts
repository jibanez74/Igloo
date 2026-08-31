import {
  expect,
  test,
  type Page,
} from "@playwright/test";

import { apiURL, readE2EEnv, type E2EEnv } from "./e2e-env";
import { trackBrowserIssues } from "./e2e-browser-issues";
import {
  readJSON,
} from "./e2e-api";
import { loginPageViaApi, logoutViaApi } from "./e2e-auth";

type AdminUser = {
  id: number;
  name: string;
  email: string;
  is_admin: boolean;
  avatar: string | null;
  created_at: string;
  updated_at: string;
};

type UsersData = {
  users: AdminUser[];
};

async function fetchAdminUsers(page: Page, env: E2EEnv) {
  const response = await page.context().request.get(
    apiURL(env, "/api/admin/users"),
    { failOnStatusCode: false },
  );
  expect(response.status()).toBe(200);

  const body = await readJSON<UsersData>(response);
  expect(body.error, body.message).toBe(false);
  return body.data?.users ?? [];
}

async function deleteUser(page: Page, env: E2EEnv, userId: number) {
  const response = await page.context().request.delete(
    apiURL(env, `/api/admin/users/${userId}`),
    { failOnStatusCode: false },
  );
  expect(response.status()).toBe(200);
}

async function cleanupAuditUsers(page: Page, env: E2EEnv, prefix: string) {
  const users = await fetchAdminUsers(page, env);
  for (const user of users) {
    if (user.email.startsWith(prefix)) {
      await deleteUser(page, env, user.id);
    }
  }
}

async function expectAppPath(page: Page, env: E2EEnv, pathname: string) {
  await expect.poll(() => new URL(page.url()).origin).toBe(
    new URL(env.baseURL).origin,
  );
  await expect.poll(() => new URL(page.url()).pathname).toBe(pathname);
}

test.describe.configure({ mode: "serial" });

test.describe("Users settings", () => {
  test("manages users accessibly without expected validation console noise", async ({
    page,
  }) => {
    const env = readE2EEnv();
    const stamp = Date.now();
    const prefix = `playwright-users-settings-${stamp}`;
    const name = `Playwright Users Settings ${stamp}`;
    const email = `${prefix}@example.com`;
    const password = `AuditPass${stamp}!`;
    const editedName = `Playwright Users Settings Edited ${stamp}`;
    const editedEmail = `${prefix}-edited@example.com`;
    const resetPassword = `ResetPass${stamp}!`;
    const tracker = trackBrowserIssues(page);
    let createPostCount = 0;
    let resetPasswordPutCount = 0;

    page.on("request", request => {
      const url = new URL(request.url());
      if (url.pathname === "/api/admin/users" && request.method() === "POST") {
        createPostCount += 1;
      }
      if (
        url.pathname.startsWith("/api/admin/users/") &&
        url.pathname.endsWith("/password") &&
        request.method() === "PUT"
      ) {
        resetPasswordPutCount += 1;
      }
    });

    await loginPageViaApi(page, env);
    await cleanupAuditUsers(page, env, prefix);

    try {
      await page.goto(apiURL(env, "/settings/users"), { waitUntil: "networkidle" });
      await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
      await expect(page.getByRole("button", { name: "Add User" })).toBeVisible();

      await page.getByRole("button", { name: "Add User" }).focus();
      await page.keyboard.press("Enter");
      await expect(page.getByRole("dialog", { name: "Add User" })).toBeVisible();

      await page.getByRole("textbox", { name: "User name" }).fill(name);
      await page.getByRole("textbox", { name: "User email" }).fill(email);
      await page.getByRole("textbox", { name: "User password" }).fill("short");
      await page.getByRole("button", { name: "Create User" }).click();
      await expect(
        page.getByRole("alert").filter({
          hasText: "Password must be at least 9 characters.",
        }),
      ).toBeVisible();
      await expect(
        page.getByRole("textbox", { name: "User password" }),
      ).toHaveAttribute("aria-invalid", "true");
      expect(createPostCount).toBe(0);

      await page.getByRole("textbox", { name: "User password" }).fill(password);
      await page.getByRole("button", { name: "Create User" }).click();
      await expect(page.getByText(name)).toBeVisible();
      expect(createPostCount).toBe(1);

      await page.getByRole("button", { name: "Add User" }).click();
      await expect(page.getByRole("dialog", { name: "Add User" })).toBeVisible();
      await page.getByRole("textbox", { name: "User name" }).fill("Duplicate User");
      await page.getByRole("textbox", { name: "User email" }).fill(email);
      await page
        .getByRole("textbox", { name: "User password" })
        .fill(`OtherPass${stamp}!`);
      await page.getByRole("button", { name: "Create User" }).click();
      await expect(
        page.getByRole("alert").filter({
          hasText: "A user with that email already exists.",
        }),
      ).toBeVisible();
      await expect(
        page.getByRole("textbox", { name: "User email" }),
      ).toHaveAttribute("aria-invalid", "true");
      expect(createPostCount).toBe(1);
      await page.getByRole("button", { name: "Cancel" }).click();
      await expect(page.getByRole("button", { name: "Add User" })).toBeFocused();

      await page.getByRole("button", { name: `Edit ${name}` }).click();
      await page.getByRole("textbox", { name: "User name" }).fill(editedName);
      await page.getByRole("textbox", { name: "User email" }).fill(editedEmail);
      await page.getByRole("checkbox", { name: "Admin privileges" }).check();
      await page.getByRole("button", { name: "Save Changes" }).click();
      const editedRow = page.locator("li").filter({ hasText: editedName });
      await expect(editedRow).toContainText("Admin");

      await page.getByRole("button", { name: `Edit ${editedName}` }).click();
      await page.getByRole("checkbox", { name: "Admin privileges" }).uncheck();
      await page.getByRole("button", { name: "Save Changes" }).click();
      await expect(editedRow).toContainText("User");

      await page
        .getByRole("button", { name: `Reset password for ${editedName}` })
        .click();
      await page
        .getByRole("textbox", { name: "New password", exact: true })
        .fill(`Mismatch${stamp}!`);
      await page
        .getByRole("textbox", { name: "Confirm new password" })
        .fill(`Different${stamp}!`);
      await expect(
        page.getByRole("alert").filter({ hasText: "Passwords do not match." }),
      ).toBeVisible();
      await expect(
        page.getByRole("button", { name: "Reset Password", exact: true }),
      ).toBeDisabled();
      expect(resetPasswordPutCount).toBe(0);
      await page
        .getByRole("textbox", { name: "New password", exact: true })
        .fill(resetPassword);
      await page
        .getByRole("textbox", { name: "Confirm new password" })
        .fill(resetPassword);
      await page.getByRole("button", { name: "Reset Password", exact: true }).click();
      await expect(page.getByRole("dialog", { name: "Reset Password" })).toBeHidden();
      expect(resetPasswordPutCount).toBe(1);

      await logoutViaApi(page.context().request, env);
      await loginPageViaApi(page, env, {
        email: editedEmail,
        password: resetPassword,
      });
      await page.goto(apiURL(env, "/settings/users"), { waitUntil: "networkidle" });
      await expectAppPath(page, env, "/settings/account");
      await expect(page.getByRole("tab", { name: "Account" })).toBeVisible();
      await expect(page.getByRole("tab", { name: "Playback" })).toBeVisible();
      await expect(page.getByRole("tab", { name: "Users" })).toHaveCount(0);

      await logoutViaApi(page.context().request, env);
      await loginPageViaApi(page, env);
      await page.goto(apiURL(env, "/settings/users"), { waitUntil: "networkidle" });
      await expect(page.getByText(editedName)).toBeVisible();

      await page.setViewportSize({ width: 360, height: 800 });
      await expect(
        page.getByRole("button", { name: `Reset password for ${editedName}` }),
      ).toBeVisible();
      await expect(
        page.getByRole("button", { name: `Edit ${editedName}` }),
      ).toBeVisible();
      await expect(
        page.getByRole("button", { name: `Delete ${editedName}` }),
      ).toBeVisible();
      await expect
        .poll(() =>
          page.evaluate(
            () => document.documentElement.scrollWidth <= window.innerWidth + 1,
          ),
        )
        .toBe(true);
      await page.setViewportSize({ width: 1440, height: 900 });

      await page.getByRole("button", { name: `Delete ${editedName}` }).click();
      await page
        .getByRole("textbox", { name: "Type DELETE to confirm user deletion" })
        .fill("delete");
      await expect(
        page.getByRole("textbox", { name: "Type DELETE to confirm user deletion" }),
      ).toHaveAttribute("aria-invalid", "true");
      await expect(page.getByRole("button", { name: "Delete User" })).toBeDisabled();
      await page
        .getByRole("textbox", { name: "Type DELETE to confirm user deletion" })
        .fill("DELETE");
      await Promise.all([
        page.waitForResponse(response => {
          const url = new URL(response.url());
          return (
            url.pathname.startsWith("/api/admin/users/") &&
            response.request().method() === "DELETE" &&
            response.status() === 200
          );
        }),
        page.getByRole("button", { name: "Delete User" }).click(),
      ]);
      await expect(page.getByRole("dialog", { name: "Delete User" })).toBeHidden();
      await expect(editedRow).toHaveCount(0);

      const deletedLogin = await page.context().request.post(
        apiURL(env, "/api/auth/login"),
        {
          data: { email: editedEmail, password: resetPassword },
          failOnStatusCode: false,
        },
      );
      expect(deletedLogin.status()).toBe(401);
    } finally {
      await logoutViaApi(page.context().request, env);
      await loginPageViaApi(page, env);
      await cleanupAuditUsers(page, env, prefix);
    }

    tracker.assertClean();
  });
});
