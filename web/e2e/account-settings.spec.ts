import {
  expect,
  test,
  type APIRequestContext,
  type APIResponse,
  type Locator,
  type Page,
  type Response,
} from "@playwright/test";

type ApiResponse<T> = {
  error: boolean;
  message?: string;
  data?: T;
};

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

type AccountSettingsEnv = {
  baseURL: string;
  email: string;
  password: string;
};

const requiredAccountControlNames = [
  "Your email address",
  "Your display name",
  "Upload avatar image",
  "Avatar image URL",
  "Current password",
  "New password",
  "Confirm new password",
  "Update Password",
  "Delete account",
];

const responsiveViewports = [
  { name: "small phone", width: 360, height: 800 },
  { name: "phone", width: 390, height: 844 },
  { name: "tablet portrait", width: 768, height: 1024 },
  { name: "tablet landscape", width: 1024, height: 768 },
  { name: "desktop", width: 1440, height: 900 },
];

function readAccountSettingsEnv(): AccountSettingsEnv {
  return {
    baseURL: process.env.E2E_BASE_URL ?? "http://localhost:3000",
    email: process.env.E2E_ADMIN_EMAIL ?? "admin@example.com",
    password: process.env.E2E_ADMIN_PASSWORD ?? "AdminPassword",
  };
}

function apiURL(env: AccountSettingsEnv, path: string) {
  return new URL(path, env.baseURL).toString();
}

function isAppApiResponse(response: Response) {
  return new URL(response.url()).pathname.startsWith("/api/");
}

async function readJSON<T>(response: APIResponse) {
  return (await response.json()) as ApiResponse<T>;
}

async function login(
  request: APIRequestContext,
  env: AccountSettingsEnv,
  email = env.email,
  password = env.password,
) {
  const response = await request.post(apiURL(env, "/api/auth/login"), {
    data: { email, password },
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(200);

  const body = await readJSON<unknown>(response);
  expect(body.error, body.message).toBe(false);
}

async function logout(request: APIRequestContext, env: AccountSettingsEnv) {
  await request.delete(apiURL(env, "/api/auth/logout"), {
    failOnStatusCode: false,
  });
}

async function fetchAdminUsers(
  request: APIRequestContext,
  env: AccountSettingsEnv,
) {
  const response = await request.get(apiURL(env, "/api/admin/users"), {
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(200);

  const body = await readJSON<UsersData>(response);
  expect(body.error, body.message).toBe(false);
  return body.data?.users ?? [];
}

async function deleteUser(
  request: APIRequestContext,
  env: AccountSettingsEnv,
  userId: number,
) {
  const response = await request.delete(
    apiURL(env, `/api/admin/users/${userId}`),
    { failOnStatusCode: false },
  );
  expect(response.status()).toBe(200);
}

async function cleanupAuditUsers(
  request: APIRequestContext,
  env: AccountSettingsEnv,
  prefix: string,
) {
  await login(request, env);
  const users = await fetchAdminUsers(request, env);
  for (const user of users) {
    if (user.email.startsWith(prefix)) {
      await deleteUser(request, env, user.id);
    }
  }
}

function trackBrowserIssues(page: Page) {
  const consoleIssues: string[] = [];
  const pageErrors: string[] = [];
  const failedRequests: string[] = [];
  const responseErrors: string[] = [];

  page.on("console", message => {
    if (message.type() === "error") {
      consoleIssues.push(`${message.type()}: ${message.text()}`);
    }
  });
  page.on("pageerror", error => pageErrors.push(error.message));
  page.on("requestfailed", request => {
    failedRequests.push(
      `${request.method()} ${request.url()} ${request.failure()?.errorText ?? ""}`,
    );
  });
  page.on("response", response => {
    if (isAppApiResponse(response) && response.status() >= 500) {
      responseErrors.push(
        `${response.status()} ${response.request().method()} ${response.url()}`,
      );
    }
  });

  return {
    assertClean() {
      expect(consoleIssues).toEqual([]);
      expect(pageErrors).toEqual([]);
      expect(failedRequests).toEqual([]);
      expect(responseErrors).toEqual([]);
    },
  };
}

async function expectDescriptionIncludes(
  page: Page,
  locator: Locator,
  expectedText: string,
) {
  const describedBy = await locator.getAttribute("aria-describedby");
  expect(describedBy).toBeTruthy();

  const descriptionText = await page.evaluate(ids => {
    return ids
      .split(/\s+/)
      .map(id => document.getElementById(id)?.textContent?.trim() ?? "")
      .filter(Boolean)
      .join(" ");
  }, describedBy ?? "");

  expect(descriptionText).toContain(expectedText);
}

async function expectAppPath(
  page: Page,
  env: AccountSettingsEnv,
  pathname: string,
) {
  await expect.poll(() => new URL(page.url()).origin).toBe(
    new URL(env.baseURL).origin,
  );
  await expect.poll(() => new URL(page.url()).pathname).toBe(pathname);
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
        noHorizontalOverflow:
          document.documentElement.scrollWidth <= window.innerWidth + 1,
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

    expect(
      audit.noHorizontalOverflow,
      `${viewport.name} must not horizontally overflow`,
    ).toBe(true);
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
    const env = readAccountSettingsEnv();
    const tracker = trackBrowserIssues(page);
    const stamp = Date.now();
    const prefix = `playwright-account-settings-${stamp}`;
    const name = `Playwright Account Settings ${stamp}`;
    const email = `${prefix}@example.com`;
    const password = `AccountPass${stamp}!`;
    const editedName = `Playwright Account Settings Edited ${stamp}`;
    const editedEmail = `${prefix}-edited@example.com`;
    const newPassword = `AccountNewPass${stamp}!`;
    let avatarUploadRequestCount = 0;

    page.on("request", request => {
      if (request.url().includes("/api/user/avatar/upload")) {
        avatarUploadRequestCount += 1;
      }
    });

    await cleanupAuditUsers(request, env, prefix);

    await login(request, env);
    const createResponse = await request.post(
      apiURL(env, "/api/admin/users"),
      {
        data: {
          name,
          email,
          password,
          is_admin: false,
        },
        failOnStatusCode: false,
      },
    );
    expect(createResponse.status()).toBe(201);
    await logout(page.context().request, env);
    await login(page.context().request, env, email, password);

    try {
      await page.goto(apiURL(env, "/settings/account"), {
        waitUntil: "networkidle",
      });
      await expect(page.getByRole("heading", { name: "Settings" })).toBeVisible();
      await expect(page.getByText("Profile Information")).toBeVisible();

      await auditResponsiveAccountPage(page);
      await auditMobileTabOrder(page);
      await page.setViewportSize({ width: 1440, height: 900 });

      await page.getByRole("textbox", { name: "Your email address" }).fill(
        editedEmail,
      );
      await Promise.all([
        page.waitForResponse(
          response =>
            response.url().endsWith("/api/user/email") &&
            response.status() === 200,
        ),
        page.getByRole("button", { name: "Save" }).first().click(),
      ]);
      await expect(
        page.getByRole("textbox", { name: "Your email address" }),
      ).toHaveValue(editedEmail);

      await page.getByRole("textbox", { name: "Your display name" }).fill(
        editedName,
      );
      await Promise.all([
        page.waitForResponse(
          response =>
            response.url().endsWith("/api/user/name") &&
            response.status() === 200,
        ),
        page.getByRole("button", { name: "Save" }).nth(1).click(),
      ]);
      await expect(page.getByText(editedName).first()).toBeVisible();

      const profileResponse = await page.context().request.get(
        apiURL(env, "/api/auth/user"),
        { failOnStatusCode: false },
      );
      expect(profileResponse.status()).toBe(200);
      const profileBody = await readJSON<{ user: AdminUser }>(profileResponse);
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
      await expectDescriptionIncludes(
        page,
        uploadInput,
        "Invalid file type. Allowed: JPEG, PNG, GIF, WebP, AVIF.",
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
      await expectDescriptionIncludes(
        page,
        newPasswordInput,
        "New password must be at least 9 characters.",
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

      const updatedLoginResponse = await request.post(
        apiURL(env, "/api/auth/login"),
        {
          data: { email: editedEmail, password: newPassword },
          failOnStatusCode: false,
        },
      );
      expect(updatedLoginResponse.status()).toBe(200);

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
      await expectDescriptionIncludes(
        page,
        deleteConfirmInput,
        "Type DELETE exactly to confirm account deletion.",
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
      await expectAppPath(page, env, "/login");

      const deletedLogin = await request.post(apiURL(env, "/api/auth/login"), {
        data: { email: editedEmail, password: newPassword },
        failOnStatusCode: false,
      });
      expect(deletedLogin.status()).toBe(401);

      await login(request, env);
      const usersResponse = await request.get(apiURL(env, "/api/admin/users"), {
        failOnStatusCode: false,
      });
      expect(usersResponse.status()).toBe(200);
      const usersBody = await readJSON<UsersData>(usersResponse);
      const userStillExists = (usersBody.data?.users ?? []).some(
        user => user.email === editedEmail || user.email === email,
      );
      expect(userStillExists).toBe(false);

      tracker.assertClean();
    } finally {
      await cleanupAuditUsers(request, env, prefix);
    }
  });
});
