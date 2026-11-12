import type React from "react";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { jsonResponse, requestURL } from "../helpers/api";
import { passwordCases } from "../helpers/password-cases";
import { watchRoomInviteUsersQueryOpts } from "@/lib/query-opts";
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

function setupUsersFetch(
  users: TestAdminUser[] = defaultUsers(),
  { failList = false }: { failList?: boolean } = {},
) {
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
      if (failList) {
        return jsonResponse({ error: true, message: "internal_error" }, 500);
      }
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
  const { queryClient } = await renderRoute("/settings/users");

  await screen.findByRole("button", { name: "Add user" });
  await screen.findByText("Dana Scully");
  return { requests, queryClient };
}

function mutationRequests(requests: CapturedRequest[], method: string) {
  return requests.filter(
    request =>
      request.method === method && request.url.startsWith("/api/admin/users"),
  );
}

describe("Users settings", () => {
  for (const form of ["create", "reset"] as const) {
    it.each(passwordCases)(`validates $name in the ${form} password form`, async ({ password, error }) => {
      const user = userEvent.setup();
      const { requests } = await renderUsersRoute();
      const creating = form === "create";
      await user.click(screen.getByRole("button", {
        name: creating ? "Add user" : "Reset password for Dana Scully",
      }));
      if (creating) {
        fireEvent.change(screen.getByLabelText("Name"), { target: { value: "New User" } });
        fireEvent.change(screen.getByLabelText("Email"), { target: { value: "new@example.com" } });
      } else {
        fireEvent.change(screen.getByLabelText("Confirm new password"), { target: { value: password } });
      }
      const input = screen.getByLabelText(creating ? "Password" : "New password");
      expect(input).not.toHaveAttribute("maxlength");
      expect(input).toHaveAccessibleDescription("At least 9 characters.");
      fireEvent.change(input, { target: { value: password } });
      const button = screen.getByRole("button", { name: creating ? "Create user" : "Reset password" });
      const method = creating ? "POST" : "PUT";

      if (error) {
        if (creating) {
          await user.click(button);
        } else {
          expect(button).toBeDisabled();
          // Submission must also validate when invoked without clicking the disabled button.
          fireEvent.submit(input.closest("form")!);
        }
        expect(screen.getByRole("alert")).toHaveTextContent(`Password ${error}`);
        expect(input).toHaveAttribute("aria-invalid", "true");
        expect(input).toHaveAccessibleDescription(expect.stringContaining(`Password ${error}`));
        expect(input).toHaveFocus();
        expect(input).toHaveValue(password);
        expect(mutationRequests(requests, method)).toHaveLength(0);
      } else {
        expect(button).toBeEnabled();
        await user.click(button);
        await waitFor(() => expect(mutationRequests(requests, method)).toHaveLength(1));
        expect(mutationRequests(requests, method)[0].body).toMatchObject({ password });
      }
    });
  }

  it("words a failed user list instead of showing the server constant", async () => {
    setupUsersFetch(defaultUsers(), { failList: true });
    await renderRoute("/settings/users");

    const error = await screen.findByRole("alert");
    expect(error).toHaveTextContent("Couldn’t load users. Please try again.");
    expect(error).not.toHaveTextContent("internal_error");
  });

  it("refreshes the watch-room invite list along with the users after a create", async () => {
    const user = userEvent.setup();
    const { requests, queryClient } = await renderUsersRoute();
    const inviteListKey = watchRoomInviteUsersQueryOpts().queryKey;
    queryClient.setQueryData(inviteListKey, { error: false, data: { users: [] } });

    await user.click(screen.getByRole("button", { name: "Add user" }));
    await user.type(screen.getByLabelText("Name"), "Walter Skinner");
    await user.type(screen.getByLabelText("Email"), "skinner@example.com");
    await user.type(screen.getByLabelText("Password"), "LongEnough123");
    await user.click(screen.getByRole("button", { name: "Create user" }));

    await waitFor(() => {
      expect(mutationRequests(requests, "POST")).toHaveLength(1);
    });
    await waitFor(() => {
      expect(queryClient.getQueryState(inviteListKey)?.isInvalidated).toBe(true);
    });
  });

  it("lists people in the shared Settings column, with their role as a badge", async () => {
    await renderUsersRoute();

    const heading = screen.getByRole("heading", {
      level: 2,
      name: "User management",
    });
    const card = heading.closest('[data-slot="card"]');
    expect(card?.parentElement).toHaveClass("max-w-5xl", "space-y-6");
    // The card's action shares the header with its title.
    expect(heading.closest('[data-slot="card-header"]')).toContainElement(
      screen.getByRole("button", { name: "Add user" }),
    );

    const list = screen.getByRole("list", { name: "User list" });
    const badges = list.querySelectorAll('[data-slot="badge"]');
    expect(badges.length).toBeGreaterThan(0);
    for (const badge of badges) {
      expect(badge.textContent).toMatch(/^(Admin|User)$/);
    }
  });

  it("offers admin rights as a real checkbox named by its visible label", async () => {
    const user = userEvent.setup();
    const { requests } = await renderUsersRoute();

    await user.click(screen.getByRole("button", { name: "Add user" }));
    const admin = screen.getByRole("checkbox", {
      name: "Grant admin privileges",
    });
    expect(admin).not.toBeChecked();
    // The label alone names it: an aria-label would hide a later rewording.
    expect(admin).not.toHaveAttribute("aria-label");

    await user.click(screen.getByText("Grant admin privileges"));
    expect(admin).toBeChecked();

    await user.type(screen.getByLabelText("Name"), "Walter Skinner");
    await user.type(screen.getByLabelText("Email"), "skinner@example.com");
    await user.type(screen.getByLabelText("Password"), "LongEnough123");
    await user.click(screen.getByRole("button", { name: "Create user" }));

    await waitFor(() => {
      expect(mutationRequests(requests, "POST")).toHaveLength(1);
    });
    expect(mutationRequests(requests, "POST")[0].body).toMatchObject({
      is_admin: true,
    });
  });

  it("names every dialog field by the label shown beside it", async () => {
    const user = userEvent.setup();
    await renderUsersRoute();

    await user.click(screen.getByRole("button", { name: "Add user" }));
    const dialog = screen.getByRole("dialog", { name: "Add user" });
    for (const name of ["Name", "Email"]) {
      const field = within(dialog).getByRole("textbox", { name });
      expect(field).not.toHaveAttribute("aria-label");
    }
    expect(within(dialog).getByLabelText("Password")).not.toHaveAttribute(
      "aria-label",
    );
  });

  it("blocks short create passwords before calling the API", async () => {
    const user = userEvent.setup();
    const { requests } = await renderUsersRoute();

    await user.click(screen.getByRole("button", { name: "Add user" }));
    await user.type(screen.getByLabelText("Name"), "Short Password User");
    await user.type(screen.getByLabelText("Email"), "short@example.com");
    await user.type(screen.getByLabelText("Password"), "short");
    await user.click(screen.getByRole("button", { name: "Create user" }));

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Password must be at least 9 characters.",
    );
    expect(screen.getByLabelText("Password")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    // Field errors stay inline (design-system §3.7); no toast repeats them.
    expect(showValidationErrorMock).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Password")).toHaveFocus();
    expect(mutationRequests(requests, "POST")).toHaveLength(0);
  });

  it("blocks duplicate create emails before calling the API", async () => {
    const user = userEvent.setup();
    const { requests } = await renderUsersRoute();

    await user.click(screen.getByRole("button", { name: "Add user" }));
    await user.type(screen.getByLabelText("Name"), "Duplicate User");
    await user.type(screen.getByLabelText("Email"), "dana@example.com");
    await user.type(screen.getByLabelText("Password"), "ValidPass123");
    await user.click(screen.getByRole("button", { name: "Create user" }));

    expect(screen.getByRole("alert")).toHaveTextContent(
      "A user with that email already exists.",
    );
    expect(screen.getByLabelText("Email")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(mutationRequests(requests, "POST")).toHaveLength(0);
  });

  it("confirms a user's deletion with Enter once DELETE is typed", async () => {
    const user = userEvent.setup();
    const { requests } = await renderUsersRoute();

    await user.click(screen.getByRole("button", { name: "Delete Dana Scully" }));
    const dialog = screen.getByRole("dialog", { name: "Delete user" });
    const confirmation = within(dialog).getByLabelText("Type DELETE to confirm");
    // Nothing is wrong with an empty field, so nothing says it is valid either.
    expect(confirmation).not.toHaveAttribute("aria-invalid");

    await user.type(confirmation, "delete{Enter}");
    expect(mutationRequests(requests, "DELETE")).toHaveLength(0);
    expect(confirmation).toHaveAttribute("aria-invalid", "true");

    await user.clear(confirmation);
    await user.type(confirmation, "DELETE{Enter}");
    await waitFor(() =>
      expect(mutationRequests(requests, "DELETE")).toHaveLength(1),
    );
  });

  it("blocks duplicate edit emails but allows the unchanged current email", async () => {
    const user = userEvent.setup();
    const { requests } = await renderUsersRoute();

    await user.click(
      screen.getByRole("button", { name: "Edit Dana Scully" }),
    );
    await user.clear(screen.getByLabelText("Email"));
    await user.type(screen.getByLabelText("Email"), "fox@example.com");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(screen.getByRole("alert")).toHaveTextContent(
      "A user with that email already exists.",
    );
    expect(mutationRequests(requests, "PATCH")).toHaveLength(0);

    await user.clear(screen.getByLabelText("Email"));
    await user.type(screen.getByLabelText("Email"), "dana@example.com");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

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
    expect(screen.getByRole("button", { name: "Reset password" })).toBeDisabled();

    await user.clear(screen.getByLabelText("New password"));
    await user.type(screen.getByLabelText("New password"), "ValidPass123");
    await user.type(
      screen.getByLabelText("Confirm new password"),
      "DifferentPass123",
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Passwords do not match.",
    );
    expect(screen.getByRole("button", { name: "Reset password" })).toBeDisabled();
    expect(mutationRequests(requests, "PUT")).toHaveLength(0);
  });

  it("restores focus to Add User after closing the Add User dialog", async () => {
    const user = userEvent.setup();
    await renderUsersRoute();

    const addUserButton = screen.getByRole("button", { name: "Add user" });
    addUserButton.focus();
    await user.keyboard("{Enter}");
    await screen.findByRole("dialog", { name: "Add user" });
    await user.keyboard("{Escape}");

    await waitFor(() => {
      expect(addUserButton).toHaveFocus();
    });

    await user.click(addUserButton);
    await screen.findByRole("dialog", { name: "Add user" });
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    await waitFor(() => {
      expect(addUserButton).toHaveFocus();
    });
  });
});
