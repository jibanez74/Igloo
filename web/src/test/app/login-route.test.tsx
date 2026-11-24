import type React from "react";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { MOVIES_IN_THEATERS_KEY, MOVIES_PER_PAGE } from "@/lib/constants";
import { authUserData } from "../helpers/fixtures";
import { jsonResponse, requestURL, stubApi } from "../helpers/api";
import { scanStatuses } from "../helpers/scan-status";
import { readDocumentHead, renderRoute } from "../helpers/render-route";

const toastMocks = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
}));

vi.mock("@/components/app/AppShell", () => ({
  default: ({ children }: { children: React.ReactNode }) => (
    <main id="main">{children}</main>
  ),
}));

vi.mock("@/lib/toast-helpers", () => ({
  showError: toastMocks.showError,
  showSuccess: toastMocks.showSuccess,
}));

function adminUser() {
  return authUserData({
    name: "Admin",
    email: "admin@example.com",
    is_admin: true,
  });
}

type MockLoginFetchOptions = {
  initialUser?: ReturnType<typeof adminUser> | null;
  loginResponse?: {
    body: unknown;
    status?: number;
  };
};

function mockLoginFetch({
  initialUser = null,
  loginResponse,
}: MockLoginFetchOptions = {}) {
  let currentUser: ReturnType<typeof adminUser> | null = initialUser;

  return stubApi({
    "GET /api/auth/user": () =>
      currentUser == null
        ? jsonResponse({ error: true, message: "not authorized" }, 401)
        : { user: currentUser },
    "POST /api/auth/login": () => {
      if (loginResponse) {
        return jsonResponse(loginResponse.body, loginResponse.status ?? 200);
      }

      currentUser = adminUser();
      return jsonResponse({
        error: false,
        message: "Hello Admin, welcome to your media library!",
      });
    },
    "/api/movies/stats": { total_movies: 0 },
    [`/api/movies/library?page=1&per_page=${MOVIES_PER_PAGE}&sort=asc`]: {
      movies: [],
      total: 0,
      page: 1,
      per_page: MOVIES_PER_PAGE,
      total_pages: 0,
    },
    "/api/tmdb/status": { available: true },
    "/api/watch-rooms": { rooms: [] },
    "/api/continue-watching": { items: [] },
    "/api/settings/scan": scanStatuses(),
    "/api/movies/latest": { movies: [] },
    "/api/shows/latest": { shows: [] },
    "/api/music/albums/latest": { albums: [] },
    "/api/tmdb/movies/in-theaters": {
      movies: [
        {
          id: 700,
          title: "Theater Fresh",
          poster_path: "",
          vote_average: 7.2,
          release_date: "2026-06-01",
        },
      ],
    },
  });
}

async function renderLoginRoute(initialEntry: string) {
  const result = await renderLoginRouteTree(initialEntry);

  await screen.findByRole("button", { name: "Sign in" });
  return result;
}

async function renderLoginRouteTree(
  initialEntry: string,
  options: MockLoginFetchOptions | ReturnType<typeof adminUser> | null = {},
) {
  const fetchOptions =
    options == null || "email" in options
      ? { initialUser: options }
      : options;
  const fetchMock = mockLoginFetch(fetchOptions);
  const { queryClient, router } = await renderRoute(initialEntry);

  return { fetchMock, queryClient, router };
}

async function signIn() {
  const user = userEvent.setup();

  await user.type(screen.getByLabelText("Email"), "admin@example.com");
  await user.type(
    screen.getByLabelText("Password", { exact: true }),
    "AdminPassword",
  );
  await user.click(screen.getByRole("button", { name: "Sign in" }));
}

describe("login route head", () => {
  it("titles and describes the sign-in page", async () => {
    await renderLoginRouteTree("/login");

    await screen.findByRole("button", { name: "Sign in" });
    expect(readDocumentHead()).toEqual({
      title: "Sign In - Igloo",
      description: "Sign in to access your personal Igloo media library.",
      descriptionCount: 1,
    });
  });
});

describe("login fields", () => {
  it("names the fields for password managers and shows the password on request", async () => {
    const user = userEvent.setup();
    await renderLoginRouteTree("/login");
    await screen.findByRole("button", { name: "Sign in" });

    const email = screen.getByLabelText("Email");
    const password = screen.getByLabelText("Password", { exact: true });
    expect(email).toHaveAttribute("autocomplete", "username");
    expect(password).toHaveAttribute("autocomplete", "current-password");
    expect(password).toHaveAttribute("type", "password");

    await user.click(screen.getByRole("button", { name: "Show password" }));
    expect(password).toHaveAttribute("type", "text");

    await user.click(screen.getByRole("button", { name: "Hide password" }));
    expect(password).toHaveAttribute("type", "password");
    expect(screen.getByRole("button", { name: "Show password" })).toBeInTheDocument();
  });
});

describe("login inline validation", () => {
  it("marks empty fields inline, focuses the first, and sends nothing", async () => {
    const user = userEvent.setup();
    const toastsBefore = toastMocks.showError.mock.calls.length;
    const { fetchMock } = await renderLoginRouteTree("/login");
    await screen.findByRole("button", { name: "Sign in" });

    await user.click(screen.getByRole("button", { name: "Sign in" }));

    const email = screen.getByLabelText("Email");
    const password = screen.getByLabelText("Password", { exact: true });
    expect(screen.getByText("Email is required.")).toHaveAttribute("role", "alert");
    expect(screen.getByText("Password is required.")).toHaveAttribute("role", "alert");
    expect(email).toHaveAttribute("aria-invalid", "true");
    expect(email).toHaveAttribute("aria-describedby", "email-error");
    expect(password).toHaveAttribute("aria-invalid", "true");
    expect(password).toHaveAttribute("aria-describedby", "password-error");
    expect(email).toHaveFocus();
    expect(toastMocks.showError.mock.calls.length).toBe(toastsBefore);
    expect(
      fetchMock.mock.calls.some(([url]) => requestURL(url) === "/api/auth/login"),
    ).toBe(false);
  });

  it("rejects a malformed email and clears the error as the reader types", async () => {
    const user = userEvent.setup();
    const { fetchMock } = await renderLoginRouteTree("/login");
    await screen.findByRole("button", { name: "Sign in" });
    const email = screen.getByLabelText("Email");

    await user.type(email, "nope");
    await user.type(screen.getByLabelText("Password", { exact: true }), "secret");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(screen.getByText("Enter a valid email address.")).toBeInTheDocument();
    expect(screen.queryByText("Password is required.")).not.toBeInTheDocument();
    expect(email).toHaveFocus();

    await user.type(email, "@example.com");

    expect(screen.queryByText("Enter a valid email address.")).not.toBeInTheDocument();
    expect(email).not.toHaveAttribute("aria-invalid");
    expect(
      fetchMock.mock.calls.some(([url]) => requestURL(url) === "/api/auth/login"),
    ).toBe(false);
  });

  it("focuses the password when it is the only missing field", async () => {
    const user = userEvent.setup();
    await renderLoginRouteTree("/login");
    await screen.findByRole("button", { name: "Sign in" });

    await user.type(screen.getByLabelText("Email"), "admin@example.com");
    await user.click(screen.getByRole("button", { name: "Sign in" }));

    expect(screen.getByText("Password is required.")).toBeInTheDocument();
    expect(screen.queryByText("Email is required.")).not.toBeInTheDocument();
    expect(screen.getByLabelText("Password", { exact: true })).toHaveFocus();
  });
});

describe("login route redirects", () => {
  it("submits an oversized password intact and words the credential error itself", async () => {
    const user = userEvent.setup();
    const { fetchMock } = await renderLoginRouteTree("/login", {
      loginResponse: { body: { error: true, message: "invalid email or password provided" }, status: 401 },
    });
    await screen.findByRole("button", { name: "Sign in" });
    const input = screen.getByLabelText("Password", { exact: true });
    const password = "a".repeat(129);
    expect(input).not.toHaveAttribute("maxlength");
    expect(input).not.toHaveAttribute("minlength");
    await user.type(screen.getByLabelText("Email"), "admin@example.com");
    await user.type(input, password);
    await user.click(screen.getByRole("button", { name: "Sign in" }));
    await waitFor(() => expect(toastMocks.showError).toHaveBeenCalledWith("Login failed", "The email or password is incorrect."));
    const request = fetchMock.mock.calls.find(([url]) => requestURL(url) === "/api/auth/login");
    expect(JSON.parse(request![1]!.body as string)).toEqual({ email: "admin@example.com", password });
  });

  it("redirects unauthenticated Home before loading in-theaters data", async () => {
    const { fetchMock, router } = await renderLoginRoute("/");

    expect(router.state.location.pathname).toBe("/login");
    expect(
      fetchMock.mock.calls.some(([input]) => {
        return requestURL(input) === "/api/tmdb/movies/in-theaters";
      }),
    ).toBe(false);
  });

  it("renders in-theaters data for authenticated Home", async () => {
    await renderLoginRouteTree("/", adminUser());

    expect(await screen.findByText("Theater Fresh")).toBeInTheDocument();
  });

  it("navigates to Home after login when no explicit redirect is provided", async () => {
    const { router } = await renderLoginRoute("/login");

    await signIn();

    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/");
    });
    expect(toastMocks.showSuccess).toHaveBeenCalledWith(
      "Welcome back!",
      "Hello Admin, welcome to your media library!",
    );
  });

  it("reenables login controls after an API error response", async () => {
    const { router } = await renderLoginRouteTree("/login", {
      loginResponse: {
        body: {
          error: true,
          message: "Invalid credentials",
        },
      },
    });

    await signIn();

    await waitFor(() => {
      expect(toastMocks.showError).toHaveBeenCalledWith(
        "Login failed",
        "Invalid credentials",
      );
    });

    expect(router.state.location.pathname).toBe("/login");
    expect(screen.getByLabelText("Email")).toBeEnabled();
    expect(screen.getByLabelText("Password", { exact: true })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Sign in" })).toBeEnabled();
  });

  it("honors an explicit safe redirect after login", async () => {
    const { router } = await renderLoginRoute(
      "/login?redirect=/settings/account",
    );

    await signIn();

    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/settings/account");
    });
  });

  it("falls back to Home after login when redirect is unsafe", async () => {
    const { router } = await renderLoginRoute(
      "/login?redirect=https://example.com",
    );

    await signIn();

    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/");
    });
  });

  it("clears stale in-theaters data before redirecting to Home after login", async () => {
    const { fetchMock, queryClient, router } = await renderLoginRoute(
      "/login?redirect=/",
    );
    queryClient.setQueryData([MOVIES_IN_THEATERS_KEY], {
      error: true,
      message: "not authorized",
    });

    await signIn();

    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/");
    });
    await waitFor(() => {
      expect(queryClient.getQueryData([MOVIES_IN_THEATERS_KEY])).toMatchObject({
        error: false,
        data: {
          movies: [
            expect.objectContaining({
              title: "Theater Fresh",
            }),
          ],
        },
      });
    });
    expect(
      fetchMock.mock.calls.some(([input]) => {
        return requestURL(input) === "/api/tmdb/movies/in-theaters";
      }),
    ).toBe(true);
  });
});
