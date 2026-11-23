import type React from "react";
import { fireEvent, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { AuthUser } from "@/types";
import { jsonResponse, requestURL, stubApi, type StubApiRequest, type StubApiRoute } from "../helpers/api";
import { authUserData } from "../helpers/fixtures";
import { passwordCases } from "../helpers/password-cases";
import { watchRoomInviteUsersQueryOpts } from "@/lib/query-opts";
import { renderRoute } from "../helpers/render-route";
import { scanStatuses } from "../helpers/scan-status";

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

type CapturedRequest = {
  method: string;
  url: string;
  body: unknown;
};

function testUser(overrides: Partial<AuthUser>): AuthUser {
  return authUserData({
    name: "Admin",
    email: "admin@example.com",
    is_admin: true,
    ...overrides,
  });
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

function requestBody(init?: RequestInit): Partial<AuthUser> | undefined {
  return typeof init?.body === "string" ? JSON.parse(init.body) : undefined;
}

function setupUsersFetch(
  users: AuthUser[] = defaultUsers(),
  {
    failList = false,
    stallListAfterDelete = false,
  }: {
    failList?: boolean;
    /** The list's refetch after a delete never answers. */
    stallListAfterDelete?: boolean;
  } = {},
) {
  let listStalled = false;
  const routes: Record<string, StubApiRoute> = {
    "/api/auth/user": () => ({ user: users[0] }),
    // The shell polls scan progress for administrators.
    "GET /api/settings/scan": scanStatuses(),
    "GET /api/admin/users": () => {
      if (failList) {
        return jsonResponse({ error: true, message: "internal_error" }, 500);
      }
      if (listStalled) return new Promise<Response>(() => {});
      return { users };
    },
    "POST /api/admin/users": ({ init }: StubApiRequest) =>
      jsonResponse(
        {
          error: false,
          data: { user: testUser({ id: 10, ...requestBody(init) }) },
        },
        201,
      ),
  };
  // stubApi keys are literal paths, so each listed user gets its own routes.
  for (const { id } of users) {
    const path = `/api/admin/users/${id}`;
    routes[`PATCH ${path}`] = ({ init }: StubApiRequest) => ({
      user: testUser({ id: 2, ...requestBody(init) }),
    });
    routes[`PUT ${path}/password`] = () =>
      jsonResponse({ error: false, message: "Password reset successfully" });
    routes[`DELETE ${path}`] = () => {
      users = users.filter(user => user.id !== id);
      listStalled = stallListAfterDelete;
      return jsonResponse({ error: false, message: "User deleted" });
    };
  }

  return stubApi(routes, { user: null });
}

type FetchMock = ReturnType<typeof stubApi>;

async function renderUsersRoute(
  users = defaultUsers(),
  options: Parameters<typeof setupUsersFetch>[1] = {},
) {
  const fetchMock = setupUsersFetch(users, options);
  const { queryClient } = await renderRoute("/settings/users");

  await screen.findByRole("button", { name: "Add user" });
  await screen.findByText("Dana Scully");
  return { fetchMock, queryClient };
}

function mutationRequests(fetchMock: FetchMock, method: string) {
  return fetchMock.mock.calls
    .map(([input, init]): CapturedRequest => ({
      method: init?.method ?? "GET",
      url: requestURL(input),
      body: requestBody(init),
    }))
    .filter(
      request =>
        request.method === method && request.url.startsWith("/api/admin/users"),
    );
}

describe("Users settings", () => {
  for (const form of ["create", "reset"] as const) {
    it.each(passwordCases)(`validates $name in the ${form} password form`, async ({ password, error }) => {
      const user = userEvent.setup();
      const { fetchMock } = await renderUsersRoute();
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
        expect(mutationRequests(fetchMock, method)).toHaveLength(0);
      } else {
        expect(button).toBeEnabled();
        await user.click(button);
        await waitFor(() => expect(mutationRequests(fetchMock, method)).toHaveLength(1));
        expect(mutationRequests(fetchMock, method)[0].body).toMatchObject({ password });
      }
    });
  }

  it("words a failed user list instead of showing the server constant", async () => {
    setupUsersFetch(defaultUsers(), { failList: true });
    await renderRoute("/settings/users");

    const error = await screen.findByRole("alert");
    expect(error).toHaveTextContent("Couldn’t load users.");
    expect(error).not.toHaveTextContent("internal_error");
  });

  it("refreshes the watch-room invite list along with the users after a create", async () => {
    const user = userEvent.setup();
    const { fetchMock, queryClient } = await renderUsersRoute();
    const inviteListKey = watchRoomInviteUsersQueryOpts().queryKey;
    queryClient.setQueryData(inviteListKey, { error: false, data: { users: [] } });

    await user.click(screen.getByRole("button", { name: "Add user" }));
    await user.type(screen.getByLabelText("Name"), "Walter Skinner");
    await user.type(screen.getByLabelText("Email"), "skinner@example.com");
    await user.type(screen.getByLabelText("Password"), "LongEnough123");
    await user.click(screen.getByRole("button", { name: "Create user" }));

    await waitFor(() => {
      expect(mutationRequests(fetchMock, "POST")).toHaveLength(1);
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
    const { fetchMock } = await renderUsersRoute();

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
      expect(mutationRequests(fetchMock, "POST")).toHaveLength(1);
    });
    expect(mutationRequests(fetchMock, "POST")[0].body).toMatchObject({
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
    const { fetchMock } = await renderUsersRoute();

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
    expect(mutationRequests(fetchMock, "POST")).toHaveLength(0);
  });

  it("blocks duplicate create emails before calling the API", async () => {
    const user = userEvent.setup();
    const { fetchMock } = await renderUsersRoute();

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
    expect(mutationRequests(fetchMock, "POST")).toHaveLength(0);
  });

  it("confirms a user's deletion with Enter once DELETE is typed", async () => {
    const user = userEvent.setup();
    const { fetchMock } = await renderUsersRoute();

    await user.click(screen.getByRole("button", { name: "Delete Dana Scully" }));
    const dialog = screen.getByRole("dialog", { name: "Delete user" });
    const confirmation = within(dialog).getByLabelText("Type DELETE to confirm");
    // Nothing is wrong with an empty field, so nothing says it is valid either.
    expect(confirmation).not.toHaveAttribute("aria-invalid");

    await user.type(confirmation, "delete{Enter}");
    expect(mutationRequests(fetchMock, "DELETE")).toHaveLength(0);
    expect(confirmation).toHaveAttribute("aria-invalid", "true");

    await user.clear(confirmation);
    await user.type(confirmation, "DELETE{Enter}");
    await waitFor(() =>
      expect(mutationRequests(fetchMock, "DELETE")).toHaveLength(1),
    );
  });

  it("blocks duplicate edit emails but allows the unchanged current email", async () => {
    const user = userEvent.setup();
    const { fetchMock } = await renderUsersRoute();

    await user.click(
      screen.getByRole("button", { name: "Edit Dana Scully" }),
    );
    await user.clear(screen.getByLabelText("Email"));
    await user.type(screen.getByLabelText("Email"), "fox@example.com");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    expect(screen.getByRole("alert")).toHaveTextContent(
      "A user with that email already exists.",
    );
    expect(mutationRequests(fetchMock, "PATCH")).toHaveLength(0);

    await user.clear(screen.getByLabelText("Email"));
    await user.type(screen.getByLabelText("Email"), "dana@example.com");
    await user.click(screen.getByRole("button", { name: "Save changes" }));

    await waitFor(() => {
      expect(mutationRequests(fetchMock, "PATCH")).toHaveLength(1);
    });
  });

  it("keeps reset password blocked with accessible password errors", async () => {
    const user = userEvent.setup();
    const { fetchMock } = await renderUsersRoute();

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
    expect(mutationRequests(fetchMock, "PUT")).toHaveLength(0);
  });

  // Focus used to return to the deleted user's own Delete button, and fall
  // to the page body when the list's refetch took the row away.
  it("moves focus to Add user once the deleted user's row is gone", async () => {
    const user = userEvent.setup();
    // The row has to leave with the delete, not with the list's refetch.
    await renderUsersRoute(defaultUsers(), { stallListAfterDelete: true });

    screen.getByRole("button", { name: "Delete Dana Scully" }).focus();
    await user.keyboard("{Enter}");
    const dialog = screen.getByRole("dialog", { name: "Delete user" });
    await user.type(
      within(dialog).getByLabelText("Type DELETE to confirm"),
      "DELETE{Enter}",
    );

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Add user" })).toHaveFocus(),
    );
    expect(screen.queryByText("Dana Scully")).not.toBeInTheDocument();
    expect(screen.getByText("Fox Mulder")).toBeInTheDocument();
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
