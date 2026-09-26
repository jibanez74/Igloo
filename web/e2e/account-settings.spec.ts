import { expect, test, type Page, type Response } from "@playwright/test";

import type { AuthUser } from "../src/types";
import { readE2EEnv } from "./e2e-env";
import {
  isExpectedUnauthorizedResourceMessage,
  trackBrowserIssues,
} from "./e2e-browser-issues";
import {
  readJSON,
} from "./e2e-api";
import { loginPageViaApi, loginViaApi, logoutViaApi } from "./e2e-auth";
import {
  expectPageHasNoHorizontalScroll,
  VIEWPORTS,
} from "./e2e-layout";
import {
  createUser,
  deleteUsersByEmailPrefix,
  fetchAdminUsers,
} from "./e2e-users";

// "Save profile" is deliberately absent: it is disabled (untabbable) until a
// profile field changes, and the save flow itself is asserted separately.
const requiredAccountControlNames = [
  "Your email address",
  "Your display name",
  "Upload avatar image",
  "Avatar image URL",
  "Current password",
  "New password",
  "Confirm new password",
  "Update Password",
  "PIN",
  "Set PIN",
  "Quick Connect code",
  "Continue",
  "Delete account",
];

const responsiveViewports = [
  { name: "small phone", width: 360, height: 800 },
  { name: "phone", ...VIEWPORTS.phone },
  { name: "tablet portrait", ...VIEWPORTS.tablet },
  { name: "tablet landscape", width: 1024, height: 768 },
  { name: "desktop", ...VIEWPORTS.desktop },
];

function isResponseTo(
  response: Response,
  method: string,
  pathname: string,
  status: number,
) {
  return (
    response.status() === status &&
    response.request().method() === method &&
    new URL(response.url()).pathname === pathname
  );
}

async function injectInvalidAvatarFile(page: Page) {
  await page.locator('input[aria-label="Upload avatar image"]').evaluate(element => {
    const input = element as HTMLInputElement;
    const dataTransfer = new DataTransfer();
    dataTransfer.items.add(
      new File(["not an image"], "avatar.txt", { type: "text/plain" }),
    );
    Object.defineProperty(input, "files", {
      configurable: true,
      value: dataTransfer.files,
    });
    input.dispatchEvent(new Event("change", { bubbles: true }));
  });
}

async function auditResponsiveAccountPage(page: Page) {
  for (const viewport of responsiveViewports) {
    await page.setViewportSize({
      width: viewport.width,
      height: viewport.height,
    });
    await expect(page.getByRole("tabpanel", { name: "Account" })).toBeVisible();

    await expectPageHasNoHorizontalScroll(page);

    const audit = await page.evaluate(requiredNames => {
      const isVisible = (element: Element) => {
        const style = window.getComputedStyle(element);
        const box = element.getBoundingClientRect();
        return (
          style.visibility !== "hidden" &&
          style.display !== "none" &&
          box.width > 0 &&
          box.height > 0
        );
      };

      const accessibleName = (element: Element) => {
        const labelledBy = (element.getAttribute("aria-labelledby") ?? "")
          .split(/\s+/)
          .filter(Boolean)
          .map(id => document.getElementById(id)?.textContent?.trim() ?? "")
          .filter(Boolean)
          .join(" ");

        if (labelledBy) {
          return labelledBy;
        }

        const ariaLabel = element.getAttribute("aria-label");
        if (ariaLabel) {
          return ariaLabel;
        }

        if (element.id) {
          const label = document.querySelector(
            `label[for="${CSS.escape(element.id)}"]`,
          );
          const labelText = label?.textContent?.trim();
          if (labelText) {
            return labelText;
          }
        }

        const wrappingLabel = element.closest("label")?.textContent?.trim();
        if (wrappingLabel) {
          return wrappingLabel;
        }

        if (element instanceof HTMLInputElement && element.placeholder) {
          return element.placeholder;
        }

        return element.textContent?.trim() ?? element.getAttribute("title") ?? "";
      };

      const interactiveElements = Array.from(
        document.querySelectorAll(
          [
            "a[href]",
            "button",
            'input:not([type="hidden"])',
            "select",
            "textarea",
            '[role="button"]',
            '[role="tab"]',
            '[role="textbox"]',
          ].join(", "),
        ),
      ).filter(isVisible);

      const unlabeled = interactiveElements
        .map(element => ({
          tag: element.tagName.toLowerCase(),
          role: element.getAttribute("role"),
          name: accessibleName(element),
        }))
        .filter(element => !element.name);

      const clippedInteractive = interactiveElements
        .map(element => {
          const box = element.getBoundingClientRect();
          return {
            name: accessibleName(element),
            left: box.left,
            right: box.right,
            width: box.width,
            height: box.height,
          };
        })
        .filter(
          box =>
            box.left < -1 ||
            box.right > window.innerWidth + 1 ||
            box.width < 1 ||
            box.height < 1,
        );

      return {
        requiredNames: Object.fromEntries(
          requiredNames.map(name => [
            name,
            interactiveElements.some(
              element => accessibleName(element) === name,
            ),
          ]),
        ),
        unlabeled,
        clippedInteractive,
      };
    }, requiredAccountControlNames);

    expect(audit.unlabeled, `${viewport.name} unlabeled controls`).toEqual([]);
    expect(
      audit.clippedInteractive,
      `${viewport.name} clipped controls`,
    ).toEqual([]);

    for (const controlName of requiredAccountControlNames) {
      expect(
        audit.requiredNames[controlName],
        `${viewport.name} missing accessible control: ${controlName}`,
      ).toBe(true);
    }
  }
}

async function activeElementName(page: Page) {
  return page.evaluate(() => {
    const active = document.activeElement;
    if (!(active instanceof HTMLElement)) {
      return "";
    }

    const labelledBy = (active.getAttribute("aria-labelledby") ?? "")
      .split(/\s+/)
      .filter(Boolean)
      .map(id => document.getElementById(id)?.textContent?.trim() ?? "")
      .filter(Boolean)
      .join(" ");

    if (labelledBy) {
      return labelledBy;
    }

    const ariaLabel = active.getAttribute("aria-label");
    if (ariaLabel) {
      return ariaLabel;
    }

    if (active.id) {
      const label = document.querySelector(
        `label[for="${CSS.escape(active.id)}"]`,
      );
      const labelText = label?.textContent?.trim();
      if (labelText) {
        return labelText;
      }
    }

    return active.textContent?.trim() ?? active.getAttribute("placeholder") ?? "";
  });
}

async function auditMobileTabOrder(page: Page) {
  await page.setViewportSize({ width: 360, height: 800 });
  await page.evaluate(() => {
    document.body.setAttribute("tabindex", "-1");
    document.body.focus();
    window.scrollTo(0, 0);
  });

  const focusedNames: string[] = [];
  for (let i = 0; i < 24; i += 1) {
    await page.keyboard.press("Tab");
    focusedNames.push(await activeElementName(page));
  }

  await page.evaluate(() => document.body.removeAttribute("tabindex"));

  for (const controlName of requiredAccountControlNames) {
    expect(focusedNames, `mobile tab order must include ${controlName}`).toContain(
      controlName,
    );
  }
}

test.describe("Account settings", () => {
  test("updates and deletes a disposable account accessibly without browser noise", async ({
    page,
    request,
  }) => {
    const env = readE2EEnv();
    const tracker = trackBrowserIssues(page, {
      ignoreConsole: (_type, text) =>
        isExpectedUnauthorizedResourceMessage(text) ||
        text ===
          "Failed to load resource: the server responded with a status of 409 (Conflict)",
      // The duplicate email is refused on purpose, and the deleted account's
      // session no longer authenticates.
      ignoreResponse: response =>
        isResponseTo(response, "PUT", "/api/user/email", 409) ||
        isResponseTo(response, "GET", "/api/auth/user", 401),
    });
    const stamp = Date.now();
    const prefix = `playwright-account-settings-${stamp}`;
    const name = `Playwright Account Settings ${stamp}`;
    const email = `${prefix}@example.com`;
    const password = `AccountPass${stamp}!`;
    const partiallyEditedName = `Playwright Account Settings Partial ${stamp}`;
    const editedName = `Playwright Account Settings Edited ${stamp}`;
    const editedEmail = `${prefix}-edited@example.com`;
    const newPassword = `AccountNewPass${stamp}!`;
    let avatarUploadRequestCount = 0;

    page.on("request", request => {
      if (request.url().includes("/api/user/avatar/upload")) {
        avatarUploadRequestCount += 1;
      }
    });

    await loginViaApi(request);
    await deleteUsersByEmailPrefix(request, prefix);
    await createUser(request, { name, email, password });
    await logoutViaApi(page.context().request);
    await loginViaApi(page.context().request, { email, password });

    try {
      await page.goto("/settings/account", {
        waitUntil: "networkidle",
      });
      await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
      await expect(page.getByText("Profile Information")).toBeVisible();

      await auditResponsiveAccountPage(page);
      await auditMobileTabOrder(page);
      await page.setViewportSize(VIEWPORTS.desktop);

      await page.getByRole("textbox", { name: "Your email address" }).fill(
        env.email,
      );
      await page.getByRole("textbox", { name: "Your display name" }).fill(
        partiallyEditedName,
      );
      await Promise.all([
        page.waitForResponse(
          response =>
            response.url().endsWith("/api/user/email") &&
            response.status() === 409,
        ),
        page.waitForResponse(
          response =>
            response.url().endsWith("/api/user/name") &&
            response.status() === 200,
        ),
        page.getByRole("button", { name: "Save profile" }).click(),
      ]);
      await expect(
        page.getByRole("alert").filter({
          hasText: /email.*already/i,
        }),
      ).toBeVisible();
      await expect(page.getByText(partiallyEditedName).first()).toBeVisible();

      const partialProfileResponse = await page.context().request.get(
        "/api/auth/user",
        { failOnStatusCode: false },
      );
      expect(partialProfileResponse.status()).toBe(200);
      const partialProfileBody = await readJSON<{ user: AuthUser }>(
        partialProfileResponse,
      );
      expect(partialProfileBody.data?.user.email).toBe(email);
      expect(partialProfileBody.data?.user.name).toBe(partiallyEditedName);

      await page.getByRole("textbox", { name: "Your email address" }).fill(
        editedEmail,
      );
      await page.getByRole("textbox", { name: "Your display name" }).fill(
        editedName,
      );
      await Promise.all([
        page.waitForResponse(
          response =>
            response.url().endsWith("/api/user/email") &&
            response.status() === 200,
        ),
        page.waitForResponse(
          response =>
            response.url().endsWith("/api/user/name") &&
            response.status() === 200,
        ),
        page.getByRole("button", { name: "Save profile" }).click(),
      ]);
      await expect(
        page.getByRole("textbox", { name: "Your email address" }),
      ).toHaveValue(editedEmail);
      await expect(page.getByText(editedName).first()).toBeVisible();

      const profileResponse = await page.context().request.get(
        "/api/auth/user",
        { failOnStatusCode: false },
      );
      expect(profileResponse.status()).toBe(200);
      const profileBody = await readJSON<{ user: AuthUser }>(profileResponse);
      expect(profileBody.data?.user.email).toBe(editedEmail);
      expect(profileBody.data?.user.name).toBe(editedName);

      await page.getByRole("textbox", { name: "Avatar image URL" }).fill(
        `data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 80 80'%3E%3Crect width='80' height='80' fill='%23f59e0b'/%3E%3C/svg%3E`,
      );
      await Promise.all([
        page.waitForResponse(
          response =>
            response.url().endsWith("/api/user/avatar") &&
            response.status() === 200,
        ),
        page.getByRole("button", { name: "Set URL" }).click(),
      ]);
      await expect(page.getByRole("img", { name: editedName })).toBeVisible();
      await expect(
        page.getByRole("textbox", { name: "Avatar image URL" }),
      ).toHaveValue("");

      await injectInvalidAvatarFile(page);
      const uploadInput = page.getByLabel("Upload avatar image");
      await expect(
        page.getByRole("alert").filter({
          hasText: "Invalid file type. Allowed: JPEG, PNG, GIF, WebP, AVIF.",
        }),
      ).toBeVisible();
      await expect(uploadInput).toHaveAttribute("aria-invalid", "true");
      await expect(uploadInput).toHaveAccessibleDescription(
        /Invalid file type\. Allowed: JPEG, PNG, GIF, WebP, AVIF\./,
      );
      expect(avatarUploadRequestCount).toBe(0);

      await page.getByLabel("Current password", { exact: true }).fill(password);
      await page.getByLabel("New password", { exact: true }).fill("short");
      await page.getByLabel("Confirm new password").fill("short");
      await page.getByRole("button", { name: "Update Password" }).click();
      const newPasswordInput = page.getByLabel("New password", { exact: true });
      await expect(
        page.getByRole("alert").filter({
          hasText: "New password must be at least 9 characters.",
        }),
      ).toBeVisible();
      await expect(newPasswordInput).toHaveAttribute("aria-invalid", "true");
      await expect(newPasswordInput).toHaveAccessibleDescription(
        /New password must be at least 9 characters\./,
      );
      await expect(newPasswordInput).toBeFocused();

      await page.getByLabel("Current password", { exact: true }).fill(password);
      await page.getByLabel("New password", { exact: true }).fill(newPassword);
      await page.getByLabel("Confirm new password").fill(newPassword);
      await Promise.all([
        page.waitForResponse(
          response =>
            response.url().endsWith("/api/user/password") &&
            response.status() === 200,
        ),
        page.getByRole("button", { name: "Update Password" }).click(),
      ]);
      await expect(page.getByLabel("Current password", { exact: true })).toHaveValue("");
      await expect(page.getByLabel("New password", { exact: true })).toHaveValue("");
      await expect(page.getByLabel("Confirm new password")).toHaveValue("");

      await loginViaApi(request, { email: editedEmail, password: newPassword });

      await page.getByRole("button", { name: "Delete account" }).click();
      const dialog = page.getByRole("dialog", { name: "Delete Account" });
      await expect(dialog).toBeVisible();
      await expect(
        page.getByLabel("Type DELETE to confirm account deletion"),
      ).toBeFocused();

      await page
        .getByLabel("Type DELETE to confirm account deletion")
        .fill("delete");
      const deleteConfirmInput = page.getByLabel(
        "Type DELETE to confirm account deletion",
      );
      await expect(deleteConfirmInput).toHaveAttribute("aria-invalid", "true");
      await expect(deleteConfirmInput).toHaveAccessibleDescription(
        /Type DELETE exactly to confirm account deletion\./,
      );
      await expect(
        page.getByRole("alert").filter({
          hasText: "Type DELETE exactly to confirm account deletion.",
        }),
      ).toBeVisible();
      await expect(
        dialog.getByRole("button", { name: "Delete Account" }),
      ).toBeDisabled();

      await page
        .getByLabel("Type DELETE to confirm account deletion")
        .fill("DELETE");
      await expect(deleteConfirmInput).not.toHaveAttribute("aria-invalid", "true");
      await expect(
        dialog.getByRole("button", { name: "Delete Account" }),
      ).toBeEnabled();
      await Promise.all([
        page.waitForResponse(
          response =>
            response.url().endsWith("/api/user") &&
            response.request().method() === "DELETE" &&
            response.status() === 200,
        ),
        dialog.getByRole("button", { name: "Delete Account" }).click(),
      ]);
      await expect(page).toHaveURL(/\/login(?:\?|$)/);

      const deletedLogin = await request.post("/api/auth/login", {
        data: { email: editedEmail, password: newPassword },
        failOnStatusCode: false,
      });
      expect(deletedLogin.status()).toBe(401);

      await loginViaApi(request);
      const userStillExists = (await fetchAdminUsers(request)).some(
        user => user.email === editedEmail || user.email === email,
      );
      expect(userStillExists).toBe(false);

      tracker.assertClean();
    } finally {
      await loginViaApi(request);
      await deleteUsersByEmailPrefix(request, prefix);
    }
  });

  test("manages the profile PIN accessibly without browser noise", async ({
    page,
    request,
  }) => {
    const tracker = trackBrowserIssues(page, {
      ignoreConsole: (_type, text) => isExpectedUnauthorizedResourceMessage(text),
      // The wrong current PIN is refused on purpose.
      ignoreResponse: response =>
        isResponseTo(response, "PUT", "/api/user/pin", 401),
    });
    const stamp = Date.now();
    const prefix = `playwright-profile-pin-${stamp}`;
    const email = `${prefix}@example.com`;
    const password = `ProfilePinPass${stamp}!`;

    await loginViaApi(request);
    await deleteUsersByEmailPrefix(request, prefix);
    await createUser(request, {
      name: `Playwright Profile PIN ${stamp}`,
      email,
      password,
    });
    await logoutViaApi(page.context().request);
    await loginViaApi(page.context().request, { email, password });

    try {
      await page.goto("/settings/account", {
        waitUntil: "networkidle",
      });
      await expect(
        page.getByRole("heading", { name: "Profile PIN" }),
      ).toBeVisible();
      await expect(
        page.getByText("You have not set a profile PIN."),
      ).toBeVisible();

      // Invalid PIN is blocked client-side with accessible errors.
      const setPinInput = page.getByLabel("PIN", { exact: true });
      await setPinInput.fill("12");
      await page.getByRole("button", { name: "Set PIN" }).click();
      await expect(
        page
          .getByRole("alert")
          .filter({ hasText: "PIN must be exactly 4 digits." }),
      ).toBeVisible();
      await expect(setPinInput).toHaveAttribute("aria-invalid", "true");
      await expect(setPinInput).toBeFocused();

      // Set the first PIN.
      await setPinInput.fill("1234");
      await Promise.all([
        page.waitForResponse(
          response =>
            response.url().endsWith("/api/user/pin") &&
            response.request().method() === "PUT" &&
            response.status() === 200,
        ),
        page.getByRole("button", { name: "Set PIN" }).click(),
      ]);

      // Reveal fetches the plaintext PIN on demand.
      const showToggle = page.getByRole("button", { name: "Show PIN" });
      await expect(showToggle).toBeVisible();
      await Promise.all([
        page.waitForResponse(
          response =>
            response.url().endsWith("/api/user/pin") &&
            response.request().method() === "GET" &&
            response.status() === 200,
        ),
        showToggle.click(),
      ]);
      await expect(page.getByText("1234")).toBeVisible();
      const hideToggle = page.getByRole("button", { name: "Hide PIN" });
      await expect(hideToggle).toHaveAttribute("aria-pressed", "true");
      await hideToggle.click();
      await expect(page.getByText("••••")).toBeVisible();

      // Changing requires the current PIN and a wrong one is rejected.
      await page.getByLabel("Current PIN").fill("0000");
      await page.getByLabel("New PIN").fill("5678");
      await Promise.all([
        page.waitForResponse(
          response =>
            response.url().endsWith("/api/user/pin") &&
            response.request().method() === "PUT" &&
            response.status() === 401,
        ),
        page.getByRole("button", { name: "Update PIN" }).click(),
      ]);
      await expect(
        page
          .getByRole("alert")
          .filter({ hasText: "current PIN is incorrect" }),
      ).toBeVisible();

      // Correct current PIN changes it.
      await page.getByLabel("Current PIN").fill("1234");
      await page.getByLabel("New PIN").fill("5678");
      await Promise.all([
        page.waitForResponse(
          response =>
            response.url().endsWith("/api/user/pin") &&
            response.request().method() === "PUT" &&
            response.status() === 200,
        ),
        page.getByRole("button", { name: "Update PIN" }).click(),
      ]);

      // Remove clears it and the card returns to first-time setup.
      await page.getByLabel("Current PIN").fill("5678");
      await Promise.all([
        page.waitForResponse(
          response =>
            response.url().endsWith("/api/user/pin") &&
            response.request().method() === "PUT" &&
            response.status() === 200,
        ),
        page.getByRole("button", { name: "Remove PIN" }).click(),
      ]);
      await expect(
        page.getByText("You have not set a profile PIN."),
      ).toBeVisible();

      tracker.assertClean();
    } finally {
      await loginViaApi(request);
      await deleteUsersByEmailPrefix(request, prefix);
    }
  });

  test("hides the danger zone from admin accounts", async ({ page }) => {
    const tracker = trackBrowserIssues(page);

    await loginPageViaApi(page);

    await page.goto("/settings/account", {
      waitUntil: "networkidle",
    });
    await expect(page.getByText("Profile Information")).toBeVisible();
    await expect(
      page.getByRole("heading", { name: "Danger Zone" }),
    ).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "Delete account" }),
    ).toHaveCount(0);

    tracker.assertClean();
  });
});
