import type React from "react";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from "@tanstack/react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MOVIES_IN_THEATERS_KEY } from "@/lib/constants";
import { routeTree } from "@/routeTree.gen";

const toastMocks = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
}));

vi.mock("@/components/AppShell", () => ({
  default: ({ children }: { children: React.ReactNode }) => (
    <main id="main">{children}</main>
  ),
}));

vi.mock("@/lib/toast-helpers", () => ({
  showError: toastMocks.showError,
  showSuccess: toastMocks.showSuccess,
}));

function jsonResponse(body: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

function requestURL(input: RequestInfo | URL) {
  if (typeof input === "string") return input;
  if (input instanceof URL) return input.toString();
  return input.url;
}

function authUser() {
  return {
    id: 1,
    name: "Admin",
    email: "admin@example.com",
    is_admin: true,
    avatar: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function mockLoginFetch(initialUser: ReturnType<typeof authUser> | null = null) {
  let currentUser: ReturnType<typeof authUser> | null = initialUser;

  const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const url = requestURL(input);
    const method = init?.method ?? "GET";

    if (url === "/api/auth/user") {
      if (currentUser == null) {
        return jsonResponse({
          error: true,
          message: "not authorized",
        }, 401);
      }

      return jsonResponse({
        error: false,
        data: { user: currentUser },
      });
    }

    if (url === "/api/auth/login" && method === "POST") {
      currentUser = authUser();
      return jsonResponse({
        error: false,
        message: "Hello Admin, welcome to your media library!",
      });
    }

    if (url === "/api/movies/stats") {
      return jsonResponse({
        error: false,
        data: { total_movies: 0 },
      });
    }

    if (url === "/api/movies/library?page=1&per_page=24&sort=asc") {
      return jsonResponse({
        error: false,
        data: {
          movies: [],
          total: 0,
          page: 1,
          per_page: 24,
          total_pages: 0,
        },
      });
    }

    if (url === "/api/tmdb/status") {
      return jsonResponse({
        error: false,
        data: { available: true },
      });
    }

    if (url === "/api/watch-rooms") {
      return jsonResponse({
        error: false,
        data: { rooms: [] },
      });
    }

    if (url === "/api/movies/latest") {
      return jsonResponse({
        error: false,
        data: { movies: [] },
      });
    }

    if (url === "/api/music/albums/latest") {
      return jsonResponse({
        error: false,
        data: { albums: [] },
      });
    }

    if (url === "/api/tmdb/movies/in-theaters") {
      return jsonResponse({
        error: false,
        data: {
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

    return jsonResponse({
      error: true,
      message: `Unexpected request: ${method} ${url}`,
    }, 500);
  });

  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

function createLoginQueryClient() {
  return new QueryClient({
    defaultOptions: {
      mutations: {
        retry: false,
      },
      queries: {
        retry: false,
      },
    },
  });
}

async function renderLoginRoute(initialEntry: string) {
  const result = await renderRoute(initialEntry);

  await screen.findByRole("button", { name: "Sign in" });
  return result;
}

async function renderRoute(
  initialEntry: string,
  initialUser: ReturnType<typeof authUser> | null = null,
) {
  vi.stubGlobal("scrollTo", vi.fn());
  const fetchMock = mockLoginFetch(initialUser);
  const queryClient = createLoginQueryClient();
  const history = createMemoryHistory({
    initialEntries: [initialEntry],
  });
  const router = createRouter({
    routeTree,
    context: {
      queryClient,
    },
    history,
  });

  await act(async () => {
    await router.load();
  });

  render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} context={{ queryClient }} />
    </QueryClientProvider>,
  );

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

afterEach(() => {
  vi.clearAllMocks();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("login route redirects", () => {
  it("redirects unauthenticated Home before loading in-theaters data", async () => {
    const { fetchMock, router } = await renderLoginRoute("/");

    expect(router.state.location.pathname).toBe("/login");
    expect(
      fetchMock.mock.calls.some(([input]) => {
        return requestURL(input as RequestInfo | URL) === "/api/tmdb/movies/in-theaters";
      }),
    ).toBe(false);
  });

  it("renders in-theaters data for authenticated Home", async () => {
    await renderRoute("/", authUser());

    expect(await screen.findByText("Theater Fresh")).toBeInTheDocument();
  });

  it("navigates to Movies after login when no explicit redirect is provided", async () => {
    const { router } = await renderLoginRoute("/login");

    await signIn();

    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/movies");
    });
    expect(toastMocks.showSuccess).toHaveBeenCalledWith(
      "Welcome back!",
      "Hello Admin, welcome to your media library!",
    );
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
        return requestURL(input as RequestInfo | URL) === "/api/tmdb/movies/in-theaters";
      }),
    ).toBe(true);
  });
});
