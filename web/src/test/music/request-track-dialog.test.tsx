import { QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  useRef,
  useState,
  type PropsWithChildren,
} from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import RequestTrackDialog from "@/components/music/RequestTrackDialog";
import { AUTH_USER_KEY } from "@/lib/constants";
import type {
  ApiResponseType,
  AuthUser,
  CreateNotificationResponseType,
  SpotifyTrackSearchResultType,
} from "@/types";
import { createTestQueryClient } from "../helpers/render";

const apiMocks = vi.hoisted(() => ({
  createNotification: vi.fn(),
  searchSpotifyTracks: vi.fn(),
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

vi.mock("@/lib/api", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/api")>("@/lib/api");

  return {
    ...actual,
    createNotification: (...args: unknown[]) =>
      apiMocks.createNotification(...args),
    searchSpotifyTracks: (...args: unknown[]) =>
      apiMocks.searchSpotifyTracks(...args),
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
  return createTestQueryClient();
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

function notificationResponse(): ApiResponseType<CreateNotificationResponseType> {
  return success({
    notification: {
      id: 1,
      created_by_user_id: 9,
      title: "track_request",
      message: "Requester: Music Fan",
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
        <RequestTrackDialog
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
  document
    .querySelectorAll('[data-testid="toast"]')
    .forEach(element => element.remove());
});

describe("RequestTrackDialog", () => {
  it("focuses the track title field and keeps Send Request disabled before selection", async () => {
    renderDialog();

    const titleInput = screen.getByLabelText("Track title");

    await waitFor(() => {
      expect(titleInput).toHaveFocus();
    });
    expect(screen.getByRole("button", { name: "Send Request" })).toBeDisabled();
  });

  it("searches Spotify and submits a track request notification", async () => {
    apiMocks.searchSpotifyTracks.mockResolvedValue(
      success({
        results: [
          spotifyTrackResult({
            spotify_id: "track456",
            title: "Hazel City",
            artist_names: ["Nina Vega"],
            album_name: "City Lights",
            spotify_url: "https://open.spotify.com/track/track456",
          }),
        ],
      }),
    );
    apiMocks.createNotification.mockResolvedValue(notificationResponse());

    const user = userEvent.setup();
    const { onOpenChange } = renderDialog();

    await user.type(screen.getByLabelText("Track title"), "Hazel City");
    await user.click(screen.getByRole("button", { name: "Search Spotify" }));

    await waitFor(() => {
      expect(apiMocks.searchSpotifyTracks).toHaveBeenCalledWith({
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
        title: "track_request",
        isAdmin: true,
        message: [
          "Requester: Music Fan",
          "Track: Hazel City",
          "Artists: Nina Vega",
          "Album: City Lights",
          "Spotify ID: track456",
          "Spotify URL: https://open.spotify.com/track/track456",
        ].join("\n"),
      });
    });

    await waitFor(() => {
      expect(onOpenChange).toHaveBeenCalledWith(false);
    });
    expect(screen.queryByRole("dialog", { name: "Request Track" }))
      .not.toBeInTheDocument();

    expect(
      await screen.findByText(
        /"Hazel City" was sent to the admin notification queue/,
      ),
    ).toBeVisible();
    expect(toastMocks.showCreated).toHaveBeenCalledWith(
      "Track request",
      "\"Hazel City\" was sent to the admin notification queue.",
    );

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "More options" })).toHaveFocus();
    });
  });

  it("clears the search loading state when Spotify search rejects", async () => {
    apiMocks.searchSpotifyTracks.mockRejectedValue(new Error("network down"));

    const user = userEvent.setup();
    renderDialog();

    await user.type(screen.getByLabelText("Track title"), "Blue Light");
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

  it("clears the confirm loading state when sending the track request rejects", async () => {
    apiMocks.searchSpotifyTracks.mockResolvedValue(
      success({
        results: [spotifyTrackResult()],
      }),
    );
    apiMocks.createNotification.mockRejectedValue(new Error("network down"));

    const user = userEvent.setup();
    renderDialog();

    await user.type(screen.getByLabelText("Track title"), "Blue Light");
    await user.click(screen.getByRole("button", { name: "Search Spotify" }));
    await user.click(await screen.findByRole("radio", { name: /Blue Light/i }));
    await user.click(screen.getByRole("button", { name: "Send Request" }));

    await waitFor(() => {
      expect(toastMocks.showActionFailed).toHaveBeenCalledWith(
        "send request",
        "Unable to complete this action right now.",
      );
    });
    expect(screen.getByRole("button", { name: "Send Request" }))
      .toBeEnabled();
    expect(screen.getByRole("dialog", { name: "Request Track" }))
      .toBeInTheDocument();
  });
});
