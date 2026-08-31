import type React from "react";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { AuthUser } from "@/types";
import { jsonResponse, requestURL } from "../helpers/api";
import { renderRoute } from "../helpers/render-route";

const showValidationErrorMock = vi.fn();
const showActionFailedMock = vi.fn();
const showSuccessMock = vi.fn();
const showErrorMock = vi.fn();

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

function setupAccountFetch(user: AuthUser = testUser()) {
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
      return jsonResponse({
        error: false,
        message: "Password updated successfully",
      });
    }

    if (url === "/api/user/avatar/upload" && method === "POST") {
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

async function renderAccountRoute(user = testUser()) {
  const { requests } = setupAccountFetch(user);
  const { router } = await renderRoute("/settings/account");

  await screen.findByText("Profile Information");
  return { requests, router };
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
    await user.click(screen.getByRole("button", { name: "Update Password" }));

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
    expect(showValidationErrorMock).toHaveBeenCalledWith(
      "New password must be at least 9 characters.",
    );
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
    await user.click(screen.getByRole("button", { name: "Update Password" }));

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("New passwords do not match.");
    expect(screen.getByLabelText("Confirm new password")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(screen.getByLabelText("Confirm new password")).toHaveAttribute(
      "aria-describedby",
      expect.stringContaining(alert.id),
    );
    expect(showValidationErrorMock).toHaveBeenCalledWith(
      "New passwords do not match.",
    );
    expect(requestsFor(requests, "PUT", "/api/user/password")).toHaveLength(0);
  });

  it("exposes invalid avatar upload validation to assistive technology", async () => {
    const { requests } = await renderAccountRoute();
    const upload = screen.getByLabelText("Upload avatar image");
    const file = new File(["not an image"], "avatar.txt", {
      type: "text/plain",
    });

    fireEvent.change(upload, { target: { files: [file] } });

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(
      "Invalid file type. Allowed: JPEG, PNG, GIF, WebP, AVIF.",
    );
    expect(upload).toHaveAttribute("aria-invalid", "true");
    expect(upload).toHaveAttribute(
      "aria-describedby",
      expect.stringContaining(alert.id),
    );
    expect(showValidationErrorMock).toHaveBeenCalledWith(
      "Invalid file type. Allowed: JPEG, PNG, GIF, WebP, AVIF.",
    );
    expect(requestsFor(requests, "POST", "/api/user/avatar/upload")).toHaveLength(
      0,
    );
  });

  it("deletes the account without calling logout after the session is destroyed", async () => {
    const user = userEvent.setup();
    const { requests, router } = await renderAccountRoute();

    await user.click(screen.getByRole("button", { name: "Delete account" }));
    const dialog = screen.getByRole("dialog", { name: "Delete Account" });
    const confirmation = within(dialog).getByLabelText(
      "Type DELETE to confirm account deletion",
    );
    await user.type(confirmation, "delete");

    expect(confirmation).toHaveAttribute("aria-invalid", "true");
    expect(
      within(dialog).getByRole("button", { name: "Delete Account" }),
    ).toBeDisabled();

    await user.clear(confirmation);
    await user.type(confirmation, "DELETE");
    await user.click(
      within(dialog).getByRole("button", { name: "Delete Account" }),
    );

    await waitFor(() =>
      expect(router.state.location.pathname).toBe("/login"),
    );
    expect(requestsFor(requests, "DELETE", "/api/user")).toHaveLength(1);
    expect(requestsFor(requests, "DELETE", "/api/auth/logout")).toHaveLength(0);
    expect(requestsFor(requests, "GET", "/api/auth/user").length).toBeGreaterThan(
      0,
    );
  });
});
