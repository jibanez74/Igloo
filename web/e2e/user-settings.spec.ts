import { expect, test } from "@playwright/test";

import { trackBrowserIssues } from "./e2e-browser-issues";
import { loginPageViaApi, logoutViaApi } from "./e2e-auth";
import { expectPageHasNoHorizontalScroll, VIEWPORTS } from "./e2e-layout";
import { deleteUsersByEmailPrefix, fetchAdminUsers } from "./e2e-users";

test.describe("Users settings", () => {
  test("manages users accessibly without expected validation console noise", async ({
    page,
  }) => {
    const stamp = Date.now();
    const prefix = `playwright-users-settings-${stamp}`;
    const name = `Playwright Users Settings ${stamp}`;
    const email = `${prefix}@example.com`;
    const password = `AuditPass${stamp}!`;
    const editedName = `Playwright Users Settings Edited ${stamp}`;
    const editedEmail = `${prefix}-edited@example.com`;
    const resetPassword = `ResetPass${stamp}!`;
    const tracker = trackBrowserIssues(page);
    const userList = page.getByRole("list", { name: "User list" });
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

    await loginPageViaApi(page);

    try {
      await page.goto("/settings/users");
      await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
      await expect(page.getByRole("button", { name: "Add User" })).toBeVisible();

      await page.getByRole("button", { name: "Add User" }).focus();
      await page.keyboard.press("Enter");
      await expect(page.getByRole("dialog", { name: "Add User" })).toBeVisible();

      await page.getByRole("textbox", { name: "User name" }).fill(name);
      await page.getByRole("textbox", { name: "User email" }).fill(email);
      await page.getByRole("textbox", { name: "User password" }).fill(password);
      await page.getByRole("button", { name: "Create User" }).click();
      await expect(userList.getByText(name)).toBeVisible();
      expect(createPostCount).toBe(1);

      await page.getByRole("button", { name: `Edit ${name}` }).click();
      await page.getByRole("textbox", { name: "User name" }).fill(editedName);
      await page.getByRole("textbox", { name: "User email" }).fill(editedEmail);
      await page.getByRole("checkbox", { name: "Admin privileges" }).check();
      await page.getByRole("button", { name: "Save Changes" }).click();
      const editedRow = userList
        .getByRole("listitem")
        .filter({ hasText: editedName });
      await expect(editedRow).toContainText("Admin");

      await page.getByRole("button", { name: `Edit ${editedName}` }).click();
      await page.getByRole("checkbox", { name: "Admin privileges" }).uncheck();
      await page.getByRole("button", { name: "Save Changes" }).click();
      await expect(editedRow).not.toContainText("Admin");
      await expect(editedRow.getByText("User", { exact: true })).toBeVisible();
      const editedUser = (await fetchAdminUsers(page.context().request)).find(
        user => user.email === editedEmail,
      );
      expect(editedUser?.is_admin).toBe(false);

      await page
        .getByRole("button", { name: `Reset password for ${editedName}` })
        .click();
      await page
        .getByRole("textbox", { name: "New password", exact: true })
        .fill(resetPassword);
      await page
        .getByRole("textbox", { name: "Confirm new password" })
        .fill(resetPassword);
      await page.getByRole("button", { name: "Reset Password", exact: true }).click();
      await expect(page.getByRole("dialog", { name: "Reset Password" })).toBeHidden();
      expect(resetPasswordPutCount).toBe(1);

      await logoutViaApi(page.context().request);
      await loginPageViaApi(page, {
        email: editedEmail,
        password: resetPassword,
      });
      await page.goto("/settings/users");
      await expect(page).toHaveURL(/\/settings\/account(?:\?|$)/);
      await expect(page.getByRole("tab", { name: "Account" })).toBeVisible();
      await expect(page.getByRole("tab", { name: "Playback" })).toBeVisible();
      await expect(page.getByRole("tab", { name: "Users" })).toHaveCount(0);

      await logoutViaApi(page.context().request);
      await loginPageViaApi(page);
      await page.goto("/settings/users");
      await expect(userList.getByText(editedName)).toBeVisible();

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
      await expectPageHasNoHorizontalScroll(page);
      await page.setViewportSize(VIEWPORTS.desktop);

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

      const deletedLogin = await page.context().request.post("/api/auth/login", {
        data: { email: editedEmail, password: resetPassword },
        failOnStatusCode: false,
      });
      expect(deletedLogin.status()).toBe(401);
    } finally {
      await logoutViaApi(page.context().request);
      await loginPageViaApi(page);
      await deleteUsersByEmailPrefix(page.context().request, prefix);
    }

    tracker.assertClean();
  });
});
