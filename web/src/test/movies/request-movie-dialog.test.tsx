import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  useRef,
  useState,
  type PropsWithChildren,
  type ReactNode,
} from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import RequestMovieDialog from "@/components/movies/RequestMovieDialog";
import { AUTH_USER_KEY } from "@/lib/constants";
import type {
  ApiResponseType,
  AuthUser,
  CreateNotificationResponseType,
  TmdbSearchResultType,
} from "@/types";

const apiMocks = vi.hoisted(() => ({
  createNotification: vi.fn(),
  searchTmdbMovies: vi.fn(),
}));
const toastMocks = vi.hoisted(() => ({
  showActionFailed: vi.fn(),
  showCreated: vi.fn((title: string, description?: string) => {
    const toast = document.createElement("div");
    toast.setAttribute("role", "status");
    toast.dataset.testid = "toast";
    toast.textContent = description
      ? `${title} created ${description}`
      : `${title} created`;
    document.body.append(toast);
  }),
  showInfo: vi.fn(),
}));

vi.mock("@tanstack/react-router", async () => {
  const actual =
    await vi.importActual<typeof import("@tanstack/react-router")>(
      "@tanstack/react-router",
    );

  return {
    ...actual,
    Link: ({
      children,
      params,
      to,
      ...props
    }: {
      children: ReactNode;
      params?: { id?: string };
      to?: string;
    }) => {
      const href =
        typeof to === "string"
          ? to.replace("$id", params?.id ?? "")
          : "#";

      return (
        <a href={href} {...props}>
          {children}
        </a>
      );
    },
  };
});

vi.mock("@/lib/api", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/api")>("@/lib/api");

  return {
    ...actual,
    createNotification: (...args: unknown[]) =>
      apiMocks.createNotification(...args),
    searchTmdbMovies: (...args: unknown[]) =>
      apiMocks.searchTmdbMovies(...args),
  };
});

vi.mock("@/lib/toast-helpers", () => ({
  showActionFailed: toastMocks.showActionFailed,
  showCreated: toastMocks.showCreated,
  showInfo: toastMocks.showInfo,
}));

function success<T extends Record<string, unknown>>(data: T): ApiResponseType<T> {
  return {
    error: false,
    data,
  };
}

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      mutations: { retry: false },
      queries: { retry: false },
    },
  });
}

function authUser(): AuthUser {
  return {
    id: 7,
    name: "Movie Fan",
    email: "movie-fan@example.com",
    is_admin: false,
    avatar: null,
    has_pin: false,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function tmdbResult(
  overrides: Partial<TmdbSearchResultType> = {},
): TmdbSearchResultType {
  return {
    tmdb_id: 603,
    title: "The Matrix",
    release_date: "1999-03-31",
    overview: "A hacker learns the truth about reality.",
    poster_path: "/matrix.jpg",
    already_in_library: false,
    ...overrides,
  };
}

function notificationResponse(): ApiResponseType<CreateNotificationResponseType> {
  return success({
    notification: {
      id: 1,
      created_by_user_id: 7,
      title: "movie_request",
      message: "Requester: Movie Fan <movie-fan@example.com>",
      is_admin: true,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
  });
}

function renderDialog() {
  const queryClient = createQueryClient();
  queryClient.setQueryData([AUTH_USER_KEY], success({ user: authUser() }));
  const onOpenChange = vi.fn();

  function Wrapper({ children }: PropsWithChildren) {
    return (
      <QueryClientProvider client={queryClient}>
        {children}
      </QueryClientProvider>
    );
  }

  function DialogHarness() {
    const [open, setOpen] = useState(true);
    const restoreFocusRef = useRef<HTMLButtonElement | null>(null);

    return (
      <>
        <button ref={restoreFocusRef} type="button">
          More options
        </button>
        <RequestMovieDialog
          open={open}
          onOpenChange={(nextOpen) => {
            onOpenChange(nextOpen);
            setOpen(nextOpen);
          }}
          restoreFocusRef={restoreFocusRef}
        />
      </>
    );
  }

  render(
    <DialogHarness />,
    { wrapper: Wrapper },
  );

  return { onOpenChange };
}

afterEach(() => {
  vi.clearAllMocks();
  document
    .querySelectorAll('[data-testid="toast"]')
    .forEach(element => element.remove());
});

describe("RequestMovieDialog", () => {
  it("focuses the title field and keeps Send Request disabled before selection", async () => {
    renderDialog();

    const titleInput = screen.getByLabelText("Title");

    await waitFor(() => {
      expect(titleInput).toHaveFocus();
    });
    expect(screen.getByRole("button", { name: "Send Request" })).toBeDisabled();
  });

  it("searches TMDB and submits a movie request notification", async () => {
    apiMocks.searchTmdbMovies.mockResolvedValue(
      success({
        results: [
          tmdbResult({
            tmdb_id: 220289,
            title: "Coherence",
            release_date: "2014-09-19",
            overview: "Friends gather during a comet sighting.",
          }),
        ],
      }),
    );
    apiMocks.createNotification.mockResolvedValue(notificationResponse());

    const user = userEvent.setup();
    const { onOpenChange } = renderDialog();

    await user.type(screen.getByLabelText("Title"), "Coherence");
    await user.type(screen.getByLabelText("Year"), "2013");
    await user.click(screen.getByRole("button", { name: "Search TMDB" }));

    await waitFor(() => {
      expect(apiMocks.searchTmdbMovies).toHaveBeenCalledWith({
        title: "Coherence",
        year: 2013,
      });
    });

    const resultRadio = await screen.findByRole("radio", { name: /Coherence/i });
    const sendRequestButton = screen.getByRole("button", { name: "Send Request" });

    expect(sendRequestButton).toBeDisabled();

    await user.tab();
    expect(resultRadio).toHaveFocus();

    await user.keyboard("{Space}");
    await waitFor(() => {
      expect(resultRadio).toBeChecked();
      expect(sendRequestButton).toBeEnabled();
    });

    await user.click(sendRequestButton);

    await waitFor(() => {
      expect(apiMocks.createNotification).toHaveBeenCalledWith({
        title: "movie_request",
        isAdmin: true,
        message: [
          "Requester: Movie Fan <movie-fan@example.com>",
          "Movie: Coherence",
          "Year: 2014",
          "TMDB ID: 220289",
          "TMDB URL: https://www.themoviedb.org/movie/220289",
        ].join("\n"),
      });
    });

    await waitFor(() => {
      expect(onOpenChange).toHaveBeenCalledWith(false);
    });
    expect(screen.queryByRole("dialog", { name: "Request Movie" }))
      .not.toBeInTheDocument();

    expect(
      await screen.findByText(
        /"Coherence" was sent to the admin notification queue/,
      ),
    ).toBeVisible();
    expect(toastMocks.showCreated).toHaveBeenCalledWith(
      "Movie request",
      "\"Coherence\" was sent to the admin notification queue.",
    );

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More options" })).toHaveFocus();
    });
  });

  it("blocks sending a request when the movie already exists in the library", async () => {
    apiMocks.searchTmdbMovies.mockResolvedValue(
      success({
        results: [
          tmdbResult({
            tmdb_id: 603,
            title: "The Matrix",
            already_in_library: false,
          }),
          tmdbResult({
            tmdb_id: 604,
            title: "The Matrix Reloaded",
            already_in_library: true,
            library_movie_id: 22,
          }),
        ],
      }),
    );

    const user = userEvent.setup();
    renderDialog();

    await user.type(screen.getByLabelText("Title"), "The Matrix");
    await user.click(screen.getByRole("button", { name: "Search TMDB" }));

    const resultRadios = await screen.findAllByRole("radio");
    const sendRequestButton = screen.getByRole("button", { name: "Send Request" });

    await user.tab();
    expect(resultRadios[0]).toHaveFocus();

    await user.keyboard("{Space}");
    await waitFor(() => {
      expect(resultRadios[0]).toBeChecked();
      expect(sendRequestButton).toBeEnabled();
    });

    await user.keyboard("{ArrowDown}");
    expect(resultRadios[1]).toHaveFocus();
    await waitFor(() => {
      expect(resultRadios[1]).toBeChecked();
      expect(sendRequestButton).toBeDisabled();
    });

    await user.tab();

    expect(screen.getByRole("link", { name: "Open existing movie" }))
      .toHaveFocus();
    expect(screen.getByRole("link", { name: "Open existing movie" }))
      .toHaveAttribute("href", "/movies/22");
    expect(sendRequestButton).toBeDisabled();
    expect(apiMocks.createNotification).not.toHaveBeenCalled();
  });
});
