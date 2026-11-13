import type React from "react";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { authUserFrom, authUserQueryOpts } from "@/lib/query-opts";
import type { AuthUser } from "@/types";
import { deferredResponse, jsonResponse, requestURL } from "../helpers/api";
import { passwordCases } from "../helpers/password-cases";
import { renderRoute } from "../helpers/render-route";
import { mutationSettled } from "../helpers/session";

const showValidationErrorMock = vi.fn();
const showActionFailedMock = vi.fn();
const showSuccessMock = vi.fn();
const showErrorMock = vi.fn();
const showInfoMock = vi.fn();

vi.mock("@/components/app/AppShell", () => ({
  default: ({ children }: { children: React.ReactNode }) => (
    <main id="main">{children}</main>
  ),
}));

vi.mock("@/lib/toast-helpers", () => ({
  showValidationError: (...args: unknown[]) =>
    showValidationErrorMock(...args),
  showActionFailed: (...args: unknown[]) => showActionFailedMock(...args),
  showSuccess: (...args: unknown[]) => showSuccessMock(...args),
  showError: (...args: unknown[]) => showErrorMock(...args),
  showInfo: (...args: unknown[]) => showInfoMock(...args),
}));

type CapturedRequest = {
  method: string;
  url: string;
  body: unknown;
};

function testUser(overrides: Partial<AuthUser> = {}): AuthUser {
  return {
    id: 2,
    name: "Dana Scully",
    email: "dana@example.com",
    is_admin: false,
    avatar: null,
    has_pin: false,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

function isNameBody(body: unknown): body is { name: string } {
  return (
    typeof body === "object" &&
    body !== null &&
    "name" in body &&
    typeof body.name === "string"
  );
}

type AccountFetchOptions = {
  /** Answers the avatar URL request in place of the stored-copy reply. */
  avatarResponse?: Promise<Response>;
  /** Saves a name change; by default the server refuses it as taken. */
  acceptName?: boolean;
  /** Answers a password change as the server does a wrong current password. */
  rejectPassword?: boolean;
  /** Answers a password change as the server does a new password it refuses. */
  rejectNewPassword?: boolean;
  /** Answers a password change with a server error about neither field. */
  failPassword?: boolean;
  /** Answers a password change in place of the reply the flags pick. */
  passwordResponse?: Promise<Response>;
  /** Answers the avatar upload in place of the stored-copy reply. */
  uploadResponse?: Promise<Response>;
};

function setupAccountFetch(
  user: AuthUser = testUser(),
  {
    avatarResponse,
    acceptName = false,
    rejectPassword = false,
    rejectNewPassword = false,
    failPassword = false,
    passwordResponse,
    uploadResponse,
  }: AccountFetchOptions = {},
) {
  const requests: CapturedRequest[] = [];
  let currentUser: AuthUser | null = user;

  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const url = requestURL(input);
    const method = init?.method ?? "GET";
    const body =
      typeof init?.body === "string" ? JSON.parse(init.body) : undefined;
    requests.push({ method, url, body });

    if (url === "/api/auth/user") {
      if (!currentUser) {
        return jsonResponse({ error: true, message: "not authenticated" }, 401);
      }

      return jsonResponse({
        error: false,
        data: { user: currentUser },
      });
    }

    if (url === "/api/devices" && method === "GET") {
      return jsonResponse({
        error: false,
        data: { devices: [] },
      });
    }

    if (url === "/api/user/password" && method === "PUT") {
      if (passwordResponse) return passwordResponse;
      if (failPassword) {
        return jsonResponse(
          { error: true, message: "internal server error" },
          500,
        );
      }
      if (rejectNewPassword) {
        return jsonResponse(
          {
            error: true,
            message: "new password must be at least 9 characters",
          },
          400,
        );
      }
      if (rejectPassword) {
        return jsonResponse(
          { error: true, message: "current password is incorrect" },
          401,
        );
      }
      return jsonResponse({
        error: false,
        message: "Password updated successfully",
      });
    }

    if (url === "/api/user/name" && method === "PUT") {
      if (acceptName && currentUser && isNameBody(body)) {
        currentUser = { ...currentUser, name: body.name };
        return jsonResponse({ error: false, data: { user: currentUser } });
      }
      return jsonResponse(
        { error: true, message: "name is already taken" },
        409,
      );
    }

    if (url === "/api/user/avatar" && method === "PUT") {
      if (avatarResponse) return avatarResponse;
      // The server stores its own copy of the image and answers with it.
      currentUser = currentUser && {
        ...currentUser,
        avatar: "/api/static/avatars/2-stored.webp",
      };
      return jsonResponse({ error: false, data: { user: currentUser } });
    }

    if (url === "/api/user/avatar/upload" && method === "POST") {
      if (uploadResponse) return uploadResponse;
      return jsonResponse({
        error: false,
        data: { user: currentUser },
      });
    }

    if (url === "/api/user" && method === "DELETE") {
      currentUser = null;
      return jsonResponse({
        error: false,
        message: "Account deleted successfully",
      });
    }

    return jsonResponse({
      error: true,
      message: `Unexpected request: ${method} ${url}`,
    }, 500);
  });

  vi.stubGlobal("fetch", fetchMock);
  return { requests };
}

async function renderAccountRoute(
  user = testUser(),
  options: AccountFetchOptions = {},
) {
  const { requests } = setupAccountFetch(user, options);
  const { router, queryClient } = await renderRoute("/settings/account");

  await screen.findByText("Profile information");
  return { requests, router, queryClient };
}

function requestsFor(
  requests: CapturedRequest[],
  method: string,
  url: string,
) {
  return requests.filter(
    request => request.method === method && request.url === url,
  );
}

describe("Account settings", () => {
  it("keeps the signed-in user when the server refuses a name change", async () => {
    const user = userEvent.setup();
    const { requests } = await renderAccountRoute();
    const userRequestsBefore = requestsFor(requests, "GET", "/api/auth/user").length;

    const nameInput = screen.getByLabelText("Name");
    await user.clear(nameInput);
    await user.type(nameInput, "Fox Mulder");
    await user.click(screen.getByRole("button", { name: "Save profile" }));

    await waitFor(() => {
      expect(showActionFailedMock).toHaveBeenCalledWith(
        "update name",
        expect.objectContaining({ status: 409 }),
      );
    });
    // A refusal changes nothing on the server, so nothing is refetched over
    // the cached user (a network error there used to replace it).
    expect(requestsFor(requests, "GET", "/api/auth/user")).toHaveLength(
      userRequestsBefore,
    );
    expect(screen.getByText("Profile information")).toBeInTheDocument();
  });

  it("caches the avatar the server stored rather than the URL typed in", async () => {
    const user = userEvent.setup();
    const { queryClient } = await renderAccountRoute();

    await user.type(
      screen.getByLabelText("Image URL"),
      "https://example.com/me.jpg",
    );
    await user.click(screen.getByRole("button", { name: "Set URL" }));

    await waitFor(() => {
      expect(showSuccessMock).toHaveBeenCalledWith("Avatar updated successfully");
    });
    expect(
      authUserFrom(queryClient.getQueryData(authUserQueryOpts().queryKey))
        ?.avatar,
    ).toBe("/api/static/avatars/2-stored.webp");
  });

  it("drops an avatar response that arrives after the session ended", async () => {
    const user = userEvent.setup();
    const avatarRequest = deferredResponse();
    const { router, queryClient } = await renderAccountRoute(testUser(), {
      avatarResponse: avatarRequest.promise,
    });

    await user.type(
      screen.getByLabelText("Image URL"),
      "https://example.com/me.jpg",
    );
    await user.click(screen.getByRole("button", { name: "Set URL" }));
    await screen.findByRole("button", { name: "Saving..." });

    // What useEndSession does on logout; the login page then clears the cache.
    await router.navigate({
      to: "/login",
      replace: true,
      state: previous => ({ ...previous, loggedOut: true }),
    });
    await waitFor(() => {
      expect(queryClient.getQueryCache().getAll()).toEqual([]);
    });

    // Wait for the mutation itself to finish, callbacks included, rather than
    // for a fixed time a slow callback could outlast.
    const settled = mutationSettled(queryClient);
    avatarRequest.resolve(
      jsonResponse({
        error: false,
        data: { user: testUser({ avatar: "/api/static/avatars/2-stored.webp" }) },
      }),
    );
    await settled;

    expect(queryClient.getQueryCache().getAll()).toEqual([]);
    expect(showSuccessMock).not.toHaveBeenCalled();
    expect(showActionFailedMock).not.toHaveBeenCalled();
  });

  it("undoes only the avatar when the URL fails after a name change saved", async () => {
    const user = userEvent.setup();
    const avatarRequest = deferredResponse();
    const { queryClient } = await renderAccountRoute(testUser(), {
      avatarResponse: avatarRequest.promise,
      acceptName: true,
    });
    const cachedUser = () =>
      authUserFrom(queryClient.getQueryData(authUserQueryOpts().queryKey));

    await user.type(
      screen.getByLabelText("Image URL"),
      "https://example.com/me.jpg",
    );
    await user.click(screen.getByRole("button", { name: "Set URL" }));
    await waitFor(() => {
      expect(cachedUser()?.avatar).toBe("https://example.com/me.jpg");
    });

    const nameInput = screen.getByLabelText("Name");
    await user.clear(nameInput);
    await user.type(nameInput, "Fox Mulder");
    await user.click(screen.getByRole("button", { name: "Save profile" }));
    await waitFor(() => {
      expect(showSuccessMock).toHaveBeenCalledWith("Name updated successfully");
    });

    avatarRequest.resolve(
      jsonResponse({ error: true, message: "avatar unreachable" }, 422),
    );
    await waitFor(() => {
      expect(showActionFailedMock).toHaveBeenCalledWith(
        "update avatar",
        expect.objectContaining({ status: 422 }),
      );
    });

    expect(cachedUser()).toMatchObject({ name: "Fox Mulder", avatar: null });
  });

  it.each(passwordCases)("validates $name when changing a password", async ({ password, error }) => {
    const user = userEvent.setup();
    const { requests } = await renderAccountRoute();
    const input = screen.getByLabelText("New password");
    expect(input).not.toHaveAttribute("maxlength");
    expect(input).toHaveAccessibleDescription("At least 9 characters.");
    fireEvent.change(screen.getByLabelText("Current password"), { target: { value: "CurrentPass123" } });
    fireEvent.change(input, { target: { value: password } });
    fireEvent.change(screen.getByLabelText("Confirm new password"), { target: { value: password } });
    await user.click(screen.getByRole("button", { name: "Update password" }));

    if (error) {
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent(`New password ${error}`);
      expect(input).toHaveAttribute("aria-invalid", "true");
      expect(input).toHaveAccessibleDescription(expect.stringContaining(`New password ${error}`));
      expect(input).toHaveFocus();
      expect(input).toHaveValue(password);
      expect(requestsFor(requests, "PUT", "/api/user/password")).toHaveLength(0);
    } else {
      await waitFor(() => expect(requestsFor(requests, "PUT", "/api/user/password")).toHaveLength(1));
      expect(requestsFor(requests, "PUT", "/api/user/password")[0].body).toEqual({
        current_password: "CurrentPass123",
        new_password: password,
      });
    }
  });

  // The server answers a wrong current password with a 401, which here is
  // about the password, not the session.
  it("keeps a wrong current password on the form instead of signing out", async () => {
    const user = userEvent.setup();
    const { router } = await renderAccountRoute(testUser(), { rejectPassword: true });

    await user.type(screen.getByLabelText("Current password"), "WrongPass123");
    await user.type(screen.getByLabelText("New password"), "NewPass12345");
    await user.type(screen.getByLabelText("Confirm new password"), "NewPass12345");
    await user.click(screen.getByRole("button", { name: "Update password" }));

    await waitFor(() => {
      expect(screen.getByLabelText("Current password")).toHaveAttribute(
        "aria-invalid",
        "true",
      );
    });
    expect(screen.getByRole("alert")).toHaveTextContent("current password is incorrect");
    expect(router.state.location.pathname).toBe("/settings/account");
    expect(showInfoMock).not.toHaveBeenCalled();
  });

  // Every refusal used to be shown under Current password, whatever it was
  // about.
  it("puts a refusal of the new password on the new password field", async () => {
    const user = userEvent.setup();
    await renderAccountRoute(testUser(), { rejectNewPassword: true });

    await user.type(screen.getByLabelText("Current password"), "RightPass123");
    await user.type(screen.getByLabelText("New password"), "NewPass12345");
    await user.type(screen.getByLabelText("Confirm new password"), "NewPass12345");
    await user.click(screen.getByRole("button", { name: "Update password" }));

    const newPassword = screen.getByLabelText("New password");
    await waitFor(() =>
      expect(newPassword).toHaveAttribute("aria-invalid", "true"),
    );
    expect(newPassword).toHaveAccessibleDescription(
      /new password must be at least 9 characters/,
    );
    expect(newPassword).toHaveFocus();
    expect(screen.getByLabelText("Current password")).not.toHaveAttribute(
      "aria-invalid",
    );
  });

  // A refusal that arrives after the reader moved on used to pull focus
  // back to the password field.
  it("leaves focus where the reader moved while a password change ran", async () => {
    const user = userEvent.setup();
    const passwordRequest = deferredResponse();
    await renderAccountRoute(testUser(), {
      passwordResponse: passwordRequest.promise,
    });

    await user.type(screen.getByLabelText("Current password"), "WrongPass123");
    await user.type(screen.getByLabelText("New password"), "NewPass12345");
    await user.type(screen.getByLabelText("Confirm new password"), "NewPass12345");
    await user.click(screen.getByRole("button", { name: "Update password" }));
    await screen.findByRole("button", { name: /updating/i });

    const name = screen.getByRole("textbox", { name: "Name" });
    name.focus();
    passwordRequest.resolve(
      jsonResponse(
        { error: true, message: "current password is incorrect" },
        401,
      ),
    );

    const current = screen.getByLabelText("Current password");
    await waitFor(() =>
      expect(current).toHaveAttribute("aria-invalid", "true"),
    );
    expect(name).toHaveFocus();
  });

  // The button disables itself while it saves and once the profile is
  // clean, which used to drop focus on the page body.
  it("keeps focus in the profile form after Save profile", async () => {
    const user = userEvent.setup();
    await renderAccountRoute(testUser(), { acceptName: true });

    const name = screen.getByRole("textbox", { name: "Name" });
    await user.clear(name);
    await user.type(name, "Riley Renamed");
    const save = screen.getByRole("button", { name: "Save profile" });
    save.focus();
    await user.keyboard("{Enter}");

    await waitFor(() => expect(save).toBeDisabled());
    expect(name).toHaveFocus();
    await waitFor(() => expect(name).toHaveValue("Riley Renamed"));
    expect(name).toHaveFocus();
  });

  // No `maxlength`: the browser counts UTF-16 units and the limit is in
  // characters, so it would cut a valid name that holds an emoji short.
  it("marks the name as required and leaves its length to validation", async () => {
    await renderAccountRoute();

    const name = screen.getByRole("textbox", { name: "Name" });
    expect(name).toBeRequired();
    expect(name).not.toHaveAttribute("maxlength");
  });

  // A failure that is not a refusal of either password used to be shown
  // as a wrong current password.
  it("leaves both password fields valid when the server fails", async () => {
    const user = userEvent.setup();
    await renderAccountRoute(testUser(), { failPassword: true });

    await user.type(screen.getByLabelText("Current password"), "RightPass123");
    await user.type(screen.getByLabelText("New password"), "NewPass12345");
    await user.type(screen.getByLabelText("Confirm new password"), "NewPass12345");
    await user.click(screen.getByRole("button", { name: "Update password" }));

    await waitFor(() =>
      expect(showActionFailedMock).toHaveBeenCalledWith(
        "update password",
        expect.objectContaining({ status: 500 }),
      ),
    );
    expect(screen.getByLabelText("Current password")).not.toHaveAttribute(
      "aria-invalid",
    );
    expect(screen.getByLabelText("New password")).not.toHaveAttribute(
      "aria-invalid",
    );
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  // Both buttons disable themselves while they save, which dropped focus
  // on the page body in a real browser.
  it("keeps focus in the password form after Update password", async () => {
    const user = userEvent.setup();
    await renderAccountRoute();

    const current = screen.getByLabelText("Current password");
    await user.type(current, "RightPass123");
    await user.type(screen.getByLabelText("New password"), "NewPass12345");
    await user.type(screen.getByLabelText("Confirm new password"), "NewPass12345");
    screen.getByRole("button", { name: "Update password" }).focus();
    await user.keyboard("{Enter}");

    await waitFor(() =>
      expect(showSuccessMock).toHaveBeenCalledWith(
        "Password updated successfully",
      ),
    );
    expect(current).toHaveFocus();
  });

  it("keeps focus on the image URL after Set URL", async () => {
    const user = userEvent.setup();
    await renderAccountRoute();

    const url = screen.getByLabelText("Image URL");
    await user.type(url, "https://example.com/me.jpg");
    screen.getByRole("button", { name: "Set URL" }).focus();
    await user.keyboard("{Enter}");

    await waitFor(() =>
      expect(showSuccessMock).toHaveBeenCalledWith("Avatar updated successfully"),
    );
    expect(url).toHaveFocus();
  });

  // The dialog was not a form, so Enter in its one field did nothing.
  it("confirms account deletion with Enter once DELETE is typed", async () => {
    const user = userEvent.setup();
    const { requests } = await renderAccountRoute();

    await user.click(screen.getByRole("button", { name: "Delete account" }));
    const dialog = screen.getByRole("dialog", { name: "Delete account" });
    const confirmation = within(dialog).getByLabelText("Type DELETE to confirm");

    await user.type(confirmation, "delete{Enter}");
    expect(requestsFor(requests, "DELETE", "/api/user")).toHaveLength(0);

    await user.clear(confirmation);
    await user.type(confirmation, "DELETE{Enter}");
    await waitFor(() =>
      expect(requestsFor(requests, "DELETE", "/api/user")).toHaveLength(1),
    );
  });

  it("keeps the hidden username autocomplete helper labeled for password managers", async () => {
    await renderAccountRoute();

    const usernameHelper = document.querySelector(
      'input[name="username"][autocomplete="username"]',
    );

    expect(usernameHelper).toBeInstanceOf(HTMLInputElement);
    expect(usernameHelper).toHaveAttribute("type", "email");
    expect(usernameHelper).toHaveAttribute("aria-label", "Account email");
    expect(usernameHelper).toHaveAttribute("hidden");
    expect(usernameHelper).toHaveAttribute("readonly");
    expect(usernameHelper).toHaveValue("dana@example.com");
  });

  it("exposes short password validation to assistive technology", async () => {
    const user = userEvent.setup();
    const { requests } = await renderAccountRoute();

    await user.type(screen.getByLabelText("Current password"), "CurrentPass123");
    await user.type(screen.getByLabelText("New password"), "short");
    await user.type(screen.getByLabelText("Confirm new password"), "short");
    await user.click(screen.getByRole("button", { name: "Update password" }));

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(
      "New password must be at least 9 characters.",
    );
    expect(screen.getByLabelText("New password")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(screen.getByLabelText("New password")).toHaveAttribute(
      "aria-describedby",
      expect.stringContaining(alert.id),
    );
    // Field errors stay inline (design-system §3.7); no toast repeats them.
    expect(showValidationErrorMock).not.toHaveBeenCalled();
    expect(screen.getByLabelText("New password")).toHaveFocus();
    expect(requestsFor(requests, "PUT", "/api/user/password")).toHaveLength(0);
  });

  it("exposes mismatched password validation to assistive technology", async () => {
    const user = userEvent.setup();
    const { requests } = await renderAccountRoute();

    await user.type(screen.getByLabelText("Current password"), "CurrentPass123");
    await user.type(screen.getByLabelText("New password"), "NewPassword123");
    await user.type(
      screen.getByLabelText("Confirm new password"),
      "DifferentPassword123",
    );
    await user.click(screen.getByRole("button", { name: "Update password" }));

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Passwords do not match.");
    expect(screen.getByLabelText("Confirm new password")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(screen.getByLabelText("Confirm new password")).toHaveAttribute(
      "aria-describedby",
      expect.stringContaining(alert.id),
    );
    // Field errors stay inline (design-system §3.7); no toast repeats them.
    expect(showValidationErrorMock).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Confirm new password")).toHaveFocus();
    expect(requestsFor(requests, "PUT", "/api/user/password")).toHaveLength(0);
  });

  // An aria-label used to rename most of these ("Your display name" over a
  // visible "Name"), so what was seen was not what was announced or spoken
  // to voice control (WCAG 2.5.3).
  it("names every field by the label shown beside it", async () => {
    await renderAccountRoute();

    for (const name of ["Email", "Name", "Image URL"]) {
      const field = screen.getByRole("textbox", { name });
      expect(field).not.toHaveAttribute("aria-label");
    }
    for (const name of [
      "Current password",
      "New password",
      "Confirm new password",
    ]) {
      expect(screen.getByLabelText(name)).not.toHaveAttribute("aria-label");
    }
    // Two labels name the file input: the field label and the button over it.
    const upload = screen.getByLabelText("Upload image");
    expect(upload).toHaveAccessibleName("Upload image Choose file");
    expect(upload).toHaveAccessibleDescription(
      "JPEG, PNG, GIF or WebP, up to 20 MB.",
    );
  });

  it("gives the new password the short hint, above the input", async () => {
    await renderAccountRoute();

    const input = screen.getByLabelText("New password");
    expect(input).toHaveAccessibleDescription("At least 9 characters.");
    const hint = screen.getByText("At least 9 characters.");
    expect(
      hint.compareDocumentPosition(input) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(screen.queryByText(/UTF-8/)).toBeNull();
  });

  // The file dialog hands focus back to the input. Disabled for the length
  // of the upload, the input could not keep it.
  it("keeps the upload input focusable while a file uploads", async () => {
    const upload = deferredResponse();
    const { requests } = await renderAccountRoute(testUser(), {
      uploadResponse: upload.promise,
    });
    const input = screen.getByLabelText("Upload image");
    const file = new File(["image"], "avatar.png", { type: "image/png" });

    input.focus();
    fireEvent.change(input, { target: { files: [file] } });

    await waitFor(() => expect(input).toHaveAttribute("aria-disabled", "true"));
    expect(input).toBeEnabled();
    expect(input).toHaveFocus();
    // A press meanwhile does not open the file dialog again.
    expect(fireEvent.click(input)).toBe(false);
    // A second file dropped on it meanwhile starts no second upload.
    fireEvent.change(input, { target: { files: [file] } });
    expect(requestsFor(requests, "POST", "/api/user/avatar/upload")).toHaveLength(
      1,
    );

    upload.resolve(
      jsonResponse({ error: false, data: { user: testUser() } }),
    );
    await waitFor(() => expect(input).not.toHaveAttribute("aria-disabled"));
    expect(input).toHaveFocus();
  });

  it("exposes invalid avatar upload validation to assistive technology", async () => {
    const { requests } = await renderAccountRoute();
    const upload = screen.getByLabelText("Upload image");
    const file = new File(["not an image"], "avatar.txt", {
      type: "text/plain",
    });

    fireEvent.change(upload, { target: { files: [file] } });

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(
      "That file type is not supported. Use JPEG, PNG, GIF or WebP.",
    );
    expect(upload).toHaveAttribute("aria-invalid", "true");
    expect(upload).toHaveAttribute(
      "aria-describedby",
      expect.stringContaining(alert.id),
    );
    // Field errors stay inline (design-system §3.7); no toast repeats them.
    expect(showValidationErrorMock).not.toHaveBeenCalled();
    expect(upload).toHaveFocus();
    expect(requestsFor(requests, "POST", "/api/user/avatar/upload")).toHaveLength(
      0,
    );
  });

  // The page says what this person's tabs hold: a non-admin has no server
  // settings and no users to manage.
  it("describes Settings by what a non-admin can change", async () => {
    await renderAccountRoute();

    expect(
      screen.getByRole("heading", { level: 1, name: "Settings" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Manage your account and playback."),
    ).toBeInTheDocument();
    expect(screen.queryByText(/and users/)).toBeNull();
  });

  it("sits in the shared Settings column", async () => {
    await renderAccountRoute();

    const card = screen
      .getByRole("heading", { name: "Profile information" })
      .closest('[data-slot="card"]');
    expect(card?.parentElement).toHaveClass("max-w-5xl", "space-y-6");
  });

  // Removals in a danger area are outlined, not filled (design-system §3.7):
  // the filled red button is the one inside the confirmation.
  it("offers account deletion as an outlined destructive action", async () => {
    await renderAccountRoute();

    expect(
      screen.getByRole("heading", { level: 2, name: "Danger zone" }),
    ).toHaveClass("text-destructive");
    expect(
      screen.getByRole("button", { name: "Delete account" }),
    ).toHaveAttribute("data-variant", "outline-destructive");
  });

  it("hides the danger zone from admin accounts", async () => {
    await renderAccountRoute(testUser({ is_admin: true }));

    expect(
      screen.queryByRole("heading", { name: "Danger zone" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Delete account" }),
    ).not.toBeInTheDocument();
  });

  it("deletes the account without calling logout after the session is destroyed", async () => {
    const user = userEvent.setup();
    const { requests, router, queryClient } = await renderAccountRoute();

    await user.click(screen.getByRole("button", { name: "Delete account" }));
    const dialog = screen.getByRole("dialog", { name: "Delete account" });
    const confirmation = within(dialog).getByLabelText(
      "Type DELETE to confirm",
    );
    await user.type(confirmation, "delete");

    expect(confirmation).toHaveAttribute("aria-invalid", "true");
    expect(
      within(dialog).getByRole("button", { name: "Delete account" }),
    ).toBeDisabled();

    await user.clear(confirmation);
    await user.type(confirmation, "DELETE");
    await user.click(
      within(dialog).getByRole("button", { name: "Delete account" }),
    );

    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/login"),
    );
    expect(requestsFor(requests, "DELETE", "/api/user")).toHaveLength(1);
    expect(requestsFor(requests, "DELETE", "/api/auth/logout")).toHaveLength(0);
    // The reader ended this session on purpose; it is no expiry to announce.
    expect(showInfoMock).not.toHaveBeenCalled();
    // The login guard knows the session is gone, so nothing asks for the
    // user again (that 401 used to land in the cache under the old page),
    // and nothing the deleted account loaded stays cached.
    await waitFor(() => {
      expect(queryClient.getQueryCache().getAll()).toEqual([]);
    });
    const deleteIndex = requests.findIndex(
      request => request.method === "DELETE" && request.url === "/api/user",
    );
    expect(
      requests
        .slice(deleteIndex)
        .filter(request => request.url === "/api/auth/user"),
    ).toEqual([]);
  });
});
