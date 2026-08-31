import type React from "react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { jsonResponse, requestURL } from "../helpers/api";
import { renderRoute } from "../helpers/render-route";

const showValidationErrorMock = vi.fn();
const showActionFailedMock = vi.fn();
const showSuccessMock = vi.fn();

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
}));

type TestAdminUser = {
  id: number;
  name: string;
  email: string;
  is_admin: boolean;
  avatar: string | null;
  created_at: string;
  updated_at: string;
};

type CapturedRequest = {
  method: string;
  url: string;
  body: unknown;
};

function testUser(overrides: Partial<TestAdminUser>): TestAdminUser {
  return {
    id: 1,
    name: "Admin",
    email: "admin@example.com",
    is_admin: true,
    avatar: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

function defaultUsers() {
  return [
    testUser({ id: 1, name: "Admin", email: "admin@example.com" }),
    testUser({
      id: 2,
      name: "Dana Scully",
      email: "dana@example.com",
      is_admin: false,
    }),
    testUser({
      id: 3,
      name: "Fox Mulder",
      email: "fox@example.com",
      is_admin: false,
    }),
  ];
}

function setupUsersFetch(users: TestAdminUser[] = defaultUsers()) {
  const requests: CapturedRequest[] = [];
  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const url = requestURL(input);
    const method = init?.method ?? "GET";
    const body =
      typeof init?.body === "string" ? JSON.parse(init.body) : undefined;
    requests.push({ method, url, body });

    if (url === "/api/auth/user") {
      return jsonResponse({
        error: false,
        data: { user: users[0] },
      });
    }

    if (url === "/api/admin/users" && method === "GET") {
      return jsonResponse({
        error: false,
        data: { users },
      });
    }

    if (url === "/api/admin/users" && method === "POST") {
      return jsonResponse({
        error: false,
        data: {
          user: testUser({
            id: 10,
            ...(body as Partial<TestAdminUser>),
          }),
        },
      }, 201);
    }

    if (url.startsWith("/api/admin/users/") && method === "PATCH") {
      return jsonResponse({
        error: false,
        data: {
          user: testUser({
            id: 2,
            ...(body as Partial<TestAdminUser>),
          }),
        },
      });
    }

    if (url.endsWith("/password") && method === "PUT") {
      return jsonResponse({
        error: false,
        message: "Password reset successfully",
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

async function renderUsersRoute(users = defaultUsers()) {
  const { requests } = setupUsersFetch(users);
  await renderRoute("/settings/users");

  await screen.findByRole("button", { name: "Add User" });
  await screen.findByText("Dana Scully");
  return { requests };
}

function mutationRequests(requests: CapturedRequest[], method: string) {
  return requests.filter(
    request =>
      request.method === method && request.url.startsWith("/api/admin/users"),
  );
}

describe("Users settings", () => {
  it("blocks short create passwords before calling the API", async () => {
    const user = userEvent.setup();
    const { requests } = await renderUsersRoute();

    await user.click(screen.getByRole("button", { name: "Add User" }));
    await user.type(screen.getByLabelText("User name"), "Short Password User");
    await user.type(screen.getByLabelText("User email"), "short@example.com");
    await user.type(screen.getByLabelText("User password"), "short");
    await user.click(screen.getByRole("button", { name: "Create User" }));

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Password must be at least 9 characters.",
    );
    expect(screen.getByLabelText("User password")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(showValidationErrorMock).toHaveBeenCalledWith(
      "Password must be at least 9 characters.",
    );
    expect(mutationRequests(requests, "POST")).toHaveLength(0);
  });

  it("blocks duplicate create emails before calling the API", async () => {
    const user = userEvent.setup();
    const { requests } = await renderUsersRoute();

    await user.click(screen.getByRole("button", { name: "Add User" }));
    await user.type(screen.getByLabelText("User name"), "Duplicate User");
    await user.type(screen.getByLabelText("User email"), "dana@example.com");
    await user.type(screen.getByLabelText("User password"), "ValidPass123");
    await user.click(screen.getByRole("button", { name: "Create User" }));

    expect(screen.getByRole("alert")).toHaveTextContent(
      "A user with that email already exists.",
    );
    expect(screen.getByLabelText("User email")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(mutationRequests(requests, "POST")).toHaveLength(0);
  });

  it("blocks duplicate edit emails but allows the unchanged current email", async () => {
    const user = userEvent.setup();
    const { requests } = await renderUsersRoute();

    await user.click(
      screen.getByRole("button", { name: "Edit Dana Scully" }),
    );
    await user.clear(screen.getByLabelText("User email"));
    await user.type(screen.getByLabelText("User email"), "fox@example.com");
    await user.click(screen.getByRole("button", { name: "Save Changes" }));

    expect(screen.getByRole("alert")).toHaveTextContent(
      "A user with that email already exists.",
    );
    expect(mutationRequests(requests, "PATCH")).toHaveLength(0);

    await user.clear(screen.getByLabelText("User email"));
    await user.type(screen.getByLabelText("User email"), "dana@example.com");
    await user.click(screen.getByRole("button", { name: "Save Changes" }));

    await waitFor(() => {
      expect(mutationRequests(requests, "PATCH")).toHaveLength(1);
    });
  });

  it("keeps reset password blocked with accessible password errors", async () => {
    const user = userEvent.setup();
    const { requests } = await renderUsersRoute();

    await user.click(
      screen.getByRole("button", {
        name: "Reset password for Dana Scully",
      }),
    );
    await user.type(screen.getByLabelText("New password"), "short");

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Password must be at least 9 characters.",
    );
    expect(screen.getByRole("button", { name: "Reset Password" })).toBeDisabled();

    await user.clear(screen.getByLabelText("New password"));
    await user.type(screen.getByLabelText("New password"), "ValidPass123");
    await user.type(
      screen.getByLabelText("Confirm new password"),
      "DifferentPass123",
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Passwords do not match.",
    );
    expect(screen.getByRole("button", { name: "Reset Password" })).toBeDisabled();
    expect(mutationRequests(requests, "PUT")).toHaveLength(0);
  });

  it("restores focus to Add User after closing the Add User dialog", async () => {
    const user = userEvent.setup();
    await renderUsersRoute();

    const addUserButton = screen.getByRole("button", { name: "Add User" });
    addUserButton.focus();
    await user.keyboard("{Enter}");
    await screen.findByRole("dialog", { name: "Add User" });
    await user.keyboard("{Escape}");

    await waitFor(() => {
      expect(addUserButton).toHaveFocus();
    });

    await user.click(addUserButton);
    await screen.findByRole("dialog", { name: "Add User" });
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    await waitFor(() => {
      expect(addUserButton).toHaveFocus();
    });
  });
});
