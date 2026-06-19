import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  useRef,
  useState,
  type PropsWithChildren,
} from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import RequestAlbumDialog from "@/components/RequestAlbumDialog";
import { AUTH_USER_KEY } from "@/lib/constants";
import type {
  ApiResponseType,
  AuthUser,
  CreateNotificationResponseType,
  SpotifyAlbumSearchResultType,
} from "@/types";

const apiMocks = vi.hoisted(() => ({
  createNotification: vi.fn(),
  searchSpotifyAlbums: vi.fn(),
}));
const routerMocks = vi.hoisted(() => ({
  navigate: vi.fn(),
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
    useNavigate: () => routerMocks.navigate,
  };
});

vi.mock("@/lib/api", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/api")>("@/lib/api");

  return {
    ...actual,
    createNotification: (...args: unknown[]) =>
      apiMocks.createNotification(...args),
    searchSpotifyAlbums: (...args: unknown[]) =>
      apiMocks.searchSpotifyAlbums(...args),
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
    id: 9,
    name: "Music Fan",
    email: "music-fan@example.com",
    is_admin: false,
    avatar: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function spotifyAlbumResult(
  overrides: Partial<SpotifyAlbumSearchResultType> = {},
): SpotifyAlbumSearchResultType {
  return {
    spotify_id: "album123",
    title: "Blue Record",
    artist_names: ["The Band"],
    release_date: "2026-01-02",
    album_type: "album",
    total_tracks: 10,
    cover_url: "https://i.scdn.co/image/cover.jpg",
    spotify_url: "https://open.spotify.com/album/album123",
    already_in_library: false,
    ...overrides,
  };
}

function notificationResponse(): ApiResponseType<CreateNotificationResponseType> {
  return success({
    notification: {
      id: 1,
      created_by_user_id: 9,
      user_id: null,
      title: "album_request",
      message: "Requester: Music Fan <music-fan@example.com>",
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
        <RequestAlbumDialog
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

describe("RequestAlbumDialog", () => {
  it("focuses the album title field and keeps Send Request disabled before selection", async () => {
    renderDialog();

    const titleInput = screen.getByLabelText("Album title");

    await waitFor(() => {
      expect(titleInput).toHaveFocus();
    });
    expect(screen.getByRole("button", { name: "Send Request" })).toBeDisabled();
  });

  it("searches Spotify and submits an album request notification", async () => {
    apiMocks.searchSpotifyAlbums.mockResolvedValue(
      success({
        results: [
          spotifyAlbumResult({
            spotify_id: "album456",
            title: "Hazel City",
            artist_names: ["Nina Vega"],
            release_date: "2025-09-12",
            total_tracks: 12,
            spotify_url: "https://open.spotify.com/album/album456",
          }),
        ],
      }),
    );
    apiMocks.createNotification.mockResolvedValue(notificationResponse());

    const user = userEvent.setup();
    const { onOpenChange } = renderDialog();

    await user.type(screen.getByLabelText("Album title"), "Hazel City");
    await user.click(screen.getByRole("button", { name: "Search Spotify" }));

    await waitFor(() => {
      expect(apiMocks.searchSpotifyAlbums).toHaveBeenCalledWith({
        title: "Hazel City",
      });
    });

    const resultRadio = await screen.findByRole("radio", { name: /Hazel City/i });
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
        title: "album_request",
        isAdmin: true,
        message: [
          "Requester: Music Fan <music-fan@example.com>",
          "Album: Hazel City",
          "Artists: Nina Vega",
          "Release date: 2025-09-12",
          "Total tracks: 12",
          "Spotify ID: album456",
          "Spotify URL: https://open.spotify.com/album/album456",
        ].join("\n"),
      });
    });

    await waitFor(() => {
      expect(onOpenChange).toHaveBeenCalledWith(false);
    });
    expect(screen.queryByRole("dialog", { name: "Request Album" }))
      .not.toBeInTheDocument();

    expect(
      await screen.findByText(
        /"Hazel City" was sent to the admin notification queue/,
      ),
    ).toBeVisible();
    expect(toastMocks.showCreated).toHaveBeenCalledWith(
      "Album request",
      "\"Hazel City\" was sent to the admin notification queue.",
    );

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More options" })).toHaveFocus();
    });
  });

  it("redirects to the existing album instead of creating a request", async () => {
    apiMocks.searchSpotifyAlbums.mockResolvedValue(
      success({
        results: [
          spotifyAlbumResult({
            spotify_id: "album111",
            title: "Green Light",
            artist_names: ["Mia June"],
            already_in_library: false,
          }),
          spotifyAlbumResult({
            spotify_id: "album222",
            already_in_library: true,
            library_album_id: 33,
          }),
        ],
      }),
    );

    const user = userEvent.setup();
    const { onOpenChange } = renderDialog();

    await user.type(screen.getByLabelText("Album title"), "Blue Record");
    await user.click(screen.getByRole("button", { name: "Search Spotify" }));

    const resultRadios = await screen.findAllByRole("radio");
    const sendRequestButton = screen.getByRole("button", { name: "Send Request" });

    await user.tab();
    expect(resultRadios[0]).toHaveFocus();

    await user.keyboard("{ArrowDown}");
    expect(resultRadios[1]).toHaveFocus();
    await waitFor(() => {
      expect(resultRadios[1]).toBeChecked();
      expect(sendRequestButton).toBeEnabled();
    });

    expect(
      screen.getByText(/already in your library/i),
    ).toBeInTheDocument();

    await user.click(sendRequestButton);

    await waitFor(() => {
      expect(onOpenChange).toHaveBeenCalledWith(false);
    });
    expect(routerMocks.navigate).toHaveBeenCalledWith({
      to: "/music/album/$id",
      params: { id: "33" },
    });
    expect(apiMocks.createNotification).not.toHaveBeenCalled();
  });

  it("clears the search loading state when Spotify search rejects", async () => {
    apiMocks.searchSpotifyAlbums.mockRejectedValue(new Error("network down"));

    const user = userEvent.setup();
    renderDialog();

    await user.type(screen.getByLabelText("Album title"), "Blue Record");
    await user.click(screen.getByRole("button", { name: "Search Spotify" }));

    await waitFor(() => {
      expect(toastMocks.showActionFailed).toHaveBeenCalledWith(
        "search Spotify",
        "Unable to complete Spotify search right now.",
      );
    });
    expect(screen.getByRole("button", { name: "Search Spotify" }))
      .toBeEnabled();
  });

  it("clears the confirm loading state when sending the album request rejects", async () => {
    apiMocks.searchSpotifyAlbums.mockResolvedValue(
      success({
        results: [spotifyAlbumResult()],
      }),
    );
    apiMocks.createNotification.mockRejectedValue(new Error("network down"));

    const user = userEvent.setup();
    renderDialog();

    await user.type(screen.getByLabelText("Album title"), "Blue Record");
    await user.click(screen.getByRole("button", { name: "Search Spotify" }));
    await user.click(await screen.findByRole("radio", { name: /Blue Record/i }));
    await user.click(screen.getByRole("button", { name: "Send Request" }));

    await waitFor(() => {
      expect(toastMocks.showActionFailed).toHaveBeenCalledWith(
        "send request",
        "Unable to complete this action right now.",
      );
    });
    expect(screen.getByRole("button", { name: "Send Request" }))
      .toBeEnabled();
    expect(screen.getByRole("dialog", { name: "Request Album" }))
      .toBeInTheDocument();
  });
});
