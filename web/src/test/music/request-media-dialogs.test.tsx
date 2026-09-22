import { QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useRef, useState, type PropsWithChildren } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import RequestAlbumDialog from "@/components/music/RequestAlbumDialog";
import RequestTrackDialog from "@/components/music/RequestTrackDialog";
import { AUTH_USER_KEY } from "@/lib/constants";
import type {
  ApiResponseType,
  AuthUser,
  CreateNotificationResponseType,
  SpotifyAlbumSearchResultType,
  SpotifyTrackSearchResultType,
} from "@/types";
import { createTestQueryClient } from "../helpers/render";

const apiMocks = vi.hoisted(() => ({
  createNotification: vi.fn(),
  searchSpotifyAlbums: vi.fn(),
  searchSpotifyTracks: vi.fn(),
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
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");

  return {
    ...actual,
    createNotification: (...args: unknown[]) =>
      apiMocks.createNotification(...args),
    searchSpotifyAlbums: (...args: unknown[]) =>
      apiMocks.searchSpotifyAlbums(...args),
    searchSpotifyTracks: (...args: unknown[]) =>
      apiMocks.searchSpotifyTracks(...args),
  };
});

vi.mock("@/lib/toast-helpers", () => ({
  showActionFailed: toastMocks.showActionFailed,
  showCreated: toastMocks.showCreated,
  showInfo: toastMocks.showInfo,
}));

function success<T extends Record<string, unknown>>(
  data: T,
): ApiResponseType<T> {
  return { error: false, data };
}

function authUser(): AuthUser {
  return {
    id: 9,
    name: "Music Fan",
    email: "music-fan@example.com",
    is_admin: false,
    avatar: null,
    has_pin: false,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function notificationResponse(): CreateNotificationResponseType {
  return { error: false };
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
    total_tracks: 9,
    cover_url: "https://i.scdn.co/image/cover.jpg",
    spotify_url: "https://open.spotify.com/album/album123",
    already_in_library: false,
    ...overrides,
  };
}

function spotifyTrackResult(
  overrides: Partial<SpotifyTrackSearchResultType> = {},
): SpotifyTrackSearchResultType {
  return {
    spotify_id: "track123",
    title: "Blue Light",
    artist_names: ["The Band"],
    album_name: "Blue Record",
    release_date: "2026-01-02",
    duration_ms: 215000,
    cover_url: "https://i.scdn.co/image/cover.jpg",
    spotify_url: "https://open.spotify.com/track/track123",
    ...overrides,
  };
}

function renderDialog(Dialog: typeof RequestAlbumDialog) {
  const queryClient = createTestQueryClient();
  queryClient.setQueryData([AUTH_USER_KEY], success({ user: authUser() }));
  const onOpenChange = vi.fn();

  function Wrapper({ children }: PropsWithChildren) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
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
        <Dialog
          open={open}
          onOpenChange={nextOpen => {
            onOpenChange(nextOpen);
            setOpen(nextOpen);
          }}
          restoreFocusRef={restoreFocusRef}
        />
      </>
    );
  }

  render(<DialogHarness />, { wrapper: Wrapper });

  return { onOpenChange };
}

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(() => {
  document
    .querySelectorAll('[data-testid="toast"]')
    .forEach(element => element.remove());
});

// The two dialogs are one component configured twice, so the shared behaviour
// is asserted once against both.
const CASES = [
  {
    name: "RequestAlbumDialog",
    Dialog: RequestAlbumDialog,
    titleLabel: "Album title",
    dialogName: "Request Album",
    searchMock: apiMocks.searchSpotifyAlbums,
    notificationTitle: "album_request",
    toastTitle: "Album request",
    query: "Hazel City",
    results: () => [
      spotifyAlbumResult({
        spotify_id: "album456",
        title: "Hazel City",
        artist_names: ["Nina Vega"],
        release_date: "2025-09-12",
        total_tracks: 12,
        spotify_url: "https://open.spotify.com/album/album456",
      }),
    ],
    message: [
      "Requester: Music Fan <music-fan@example.com>",
      "Album: Hazel City",
      "Artists: Nina Vega",
      "Release date: 2025-09-12",
      "Total tracks: 12",
      "Spotify ID: album456",
      "Spotify URL: https://open.spotify.com/album/album456",
    ].join("\n"),
    rejectResults: () => [spotifyAlbumResult()],
    rejectRadio: /Blue Record/i,
  },
  {
    name: "RequestTrackDialog",
    Dialog: RequestTrackDialog,
    titleLabel: "Track title",
    dialogName: "Request Track",
    searchMock: apiMocks.searchSpotifyTracks,
    notificationTitle: "track_request",
    toastTitle: "Track request",
    query: "Hazel City",
    results: () => [
      spotifyTrackResult({
        spotify_id: "track456",
        title: "Hazel City",
        artist_names: ["Nina Vega"],
        album_name: "City Lights",
        spotify_url: "https://open.spotify.com/track/track456",
      }),
    ],
    message: [
      "Requester: Music Fan <music-fan@example.com>",
      "Track: Hazel City",
      "Artists: Nina Vega",
      "Album: City Lights",
      "Spotify ID: track456",
      "Spotify URL: https://open.spotify.com/track/track456",
    ].join("\n"),
    rejectResults: () => [spotifyTrackResult()],
    rejectRadio: /Blue Light/i,
  },
] as const;

describe.each(CASES)("$name", (testCase) => {
  it("focuses the title field and keeps Send Request disabled before selection", async () => {
    renderDialog(testCase.Dialog);

    const titleInput = screen.getByLabelText(testCase.titleLabel);

    await waitFor(() => {
      expect(titleInput).toHaveFocus();
    });
    expect(screen.getByRole("button", { name: "Send Request" })).toBeDisabled();
  });

  it("searches Spotify and submits the request notification", async () => {
    testCase.searchMock.mockResolvedValue(
      success({ results: testCase.results() }),
    );
    apiMocks.createNotification.mockResolvedValue(notificationResponse());

    const user = userEvent.setup();
    const { onOpenChange } = renderDialog(testCase.Dialog);

    await user.type(screen.getByLabelText(testCase.titleLabel), testCase.query);
    await user.click(screen.getByRole("button", { name: "Search Spotify" }));

    await waitFor(() => {
      expect(testCase.searchMock).toHaveBeenCalledWith({
        title: testCase.query,
      });
    });

    const resultRadio = await screen.findByRole("radio", {
      name: /Hazel City/i,
    });
    const sendRequestButton = screen.getByRole("button", {
      name: "Send Request",
    });

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
        title: testCase.notificationTitle,
        isAdmin: true,
        message: testCase.message,
      });
    });

    await waitFor(() => {
      expect(onOpenChange).toHaveBeenCalledWith(false);
    });
    expect(
      screen.queryByRole("dialog", { name: testCase.dialogName }),
    ).not.toBeInTheDocument();

    expect(
      await screen.findByText(
        /"Hazel City" was sent to the admin notification queue/,
      ),
    ).toBeVisible();
    expect(toastMocks.showCreated).toHaveBeenCalledWith(
      testCase.toastTitle,
      '"Hazel City" was sent to the admin notification queue.',
    );

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More options" })).toHaveFocus();
    });
  });

  it("clears the search loading state when Spotify search rejects", async () => {
    testCase.searchMock.mockRejectedValue(new Error("network down"));

    const user = userEvent.setup();
    renderDialog(testCase.Dialog);

    await user.type(screen.getByLabelText(testCase.titleLabel), "Blue");
    await user.click(screen.getByRole("button", { name: "Search Spotify" }));

    await waitFor(() => {
      expect(toastMocks.showActionFailed).toHaveBeenCalledWith(
        "search Spotify",
        "Unable to complete Spotify search right now.",
      );
    });
    expect(screen.getByRole("button", { name: "Search Spotify" })).toBeEnabled();
  });

  it("clears the confirm loading state when sending the request rejects", async () => {
    testCase.searchMock.mockResolvedValue(
      success({ results: testCase.rejectResults() }),
    );
    apiMocks.createNotification.mockRejectedValue(new Error("network down"));

    const user = userEvent.setup();
    renderDialog(testCase.Dialog);

    await user.type(screen.getByLabelText(testCase.titleLabel), "Blue");
    await user.click(screen.getByRole("button", { name: "Search Spotify" }));
    await user.click(
      await screen.findByRole("radio", { name: testCase.rejectRadio }),
    );
    await user.click(screen.getByRole("button", { name: "Send Request" }));

    await waitFor(() => {
      expect(toastMocks.showActionFailed).toHaveBeenCalledWith(
        "send request",
        "Unable to complete this action right now.",
      );
    });
    expect(screen.getByRole("button", { name: "Send Request" })).toBeEnabled();
    expect(
      screen.getByRole("dialog", { name: testCase.dialogName }),
    ).toBeInTheDocument();
  });
});

// The one branch only the album dialog has.
describe("RequestAlbumDialog", () => {
  it("opens the existing album instead of creating a request", async () => {
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
    const { onOpenChange } = renderDialog(RequestAlbumDialog);

    await user.type(screen.getByLabelText("Album title"), "Blue Record");
    await user.click(screen.getByRole("button", { name: "Search Spotify" }));

    const resultRadios = await screen.findAllByRole("radio");
    const sendRequestButton = screen.getByRole("button", {
      name: "Send Request",
    });

    await user.tab();
    expect(resultRadios[0]).toHaveFocus();

    await user.keyboard("{ArrowDown}");
    expect(resultRadios[1]).toHaveFocus();
    await waitFor(() => {
      expect(resultRadios[1]).toBeChecked();
      expect(sendRequestButton).toBeEnabled();
    });

    expect(screen.getByText(/already in your library/i)).toBeInTheDocument();

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
});
