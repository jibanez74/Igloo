import { act, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from "@tanstack/react-router";
import { describe, expect, it, vi } from "vitest";
import { routeTree } from "@/routeTree.gen";

function jsonResponse(body: unknown) {
  return Promise.resolve({
    status: 200,
    json: async () => body,
  } as Response);
}

function createSearchQueryClient() {
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

function mockSearchFetch() {
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url =
      typeof input === "string"
        ? input
        : input instanceof URL
          ? input.toString()
          : input.url;

    if (url === "/api/auth/user") {
      return jsonResponse({
        error: false,
        data: {
          user: {
            id: 1,
            name: "Search User",
            email: "search@example.com",
            is_admin: false,
            avatar: null,
            created_at: "2026-01-01T00:00:00Z",
            updated_at: "2026-01-01T00:00:00Z",
          },
        },
      });
    }

    if (url.startsWith("/api/search/movies?")) {
      return jsonResponse({
        error: false,
        data: {
          query: "Casino",
          results: [
            {
              id: 7,
              title: "Casino Royale",
              poster_path: { String: "", Valid: false },
              year: { Int64: 2006, Valid: true },
              certification: { String: "PG-13", Valid: true },
            },
          ],
          total: 50,
          page: 3,
          per_page: 24,
          total_pages: 3,
        },
      });
    }

    return jsonResponse({
      error: true,
      message: `Unexpected request: ${url}`,
    });
  });

  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

async function renderSearchRoute(initialEntry: string) {
  const fetchMock = mockSearchFetch();
  const queryClient = createSearchQueryClient();
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

  return { fetchMock, router };
}

describe("search route", () => {
  it("shows the server-clamped page for overlarge requested pages", async () => {
    window.scrollTo = vi.fn();
    const { fetchMock } = await renderSearchRoute(
      "/search/?q=Casino&tab=movies&page=999",
    );

    expect(await screen.findByText("Casino Royale")).toBeInTheDocument();
    expect(screen.getByText("50 movies")).toBeInTheDocument();
    expect(screen.getByText("Page 3 of 3")).toBeInTheDocument();

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        "/api/search/movies?q=Casino&page=999&per_page=24",
        expect.objectContaining({
          credentials: "include",
          method: "GET",
        }),
      );
    });
  });
});
