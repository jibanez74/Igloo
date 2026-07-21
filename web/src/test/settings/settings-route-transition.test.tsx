import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from "@tanstack/react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AudioPlayerStateContext } from "@/context/AudioPlayerContext";
import {
  GENERAL_SETTINGS_KEY,
  MOTION_CONTROL_THUMB_TRANSFORM_CLASS,
  MOTION_SETTINGS_SURFACE_CLASS,
  PLAYBACK_SETTINGS_KEY,
  SETTINGS_KEY,
} from "@/lib/constants";
import { routeTree } from "@/routeTree.gen";
import type { AudioPlayerState } from "@/types";
import { runContentFadeTransitionTimeout } from "../helpers/content-fade-transition";

const defaultMatchMedia = window.matchMedia;
const originalStartViewTransition = (document as Document & {
  startViewTransition?: unknown;
}).startViewTransition;

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
    name: "Settings User",
    email: "settings@example.com",
    is_admin: true,
    avatar: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

function generalSettings() {
  return {
    tmdb_key: null,
    immich_base_url: null,
    immich_api_key: null,
    jellyfin_base_url: null,
    jellyfin_api_key: null,
    spotify_client_id: null,
    spotify_client_secret: null,
    enable_watcher: false,
    download_images: true,
    static_dir: "/var/lib/igloo/static",
    transcode_dir: "/var/lib/igloo/transcode",
  };
}

function playbackSettings() {
  return {
    profiles: [
      {
        id: "720p_4mbps",
        label: "720p - 4 Mbps",
        height: 720,
        video_mbps: 4,
      },
      {
        id: "1080p_8mbps",
        label: "1080p - 8 Mbps",
        height: 1080,
        video_mbps: 8,
      },
    ],
    preferred_profile: null,
    download_mbps: null,
    server_upload_mbps: 20,
    hardware_acceleration_device: "cpu",
    is_admin: true,
    preferred_audio_language: null,
    preferred_subtitle_language: null,
  };
}

function librarySettings() {
  return {
    movies_dir: "/media/movies",
    shows_dir: "/media/shows",
    music_dir: "/media/music",
  };
}

function mockSettingsFetch() {
  const fetchMock = vi.fn((input: RequestInfo | URL) => {
    const url = requestURL(input);

    if (url === "/api/auth/user") {
      return jsonResponse({
        error: false,
        data: {
          user: authUser(),
        },
      });
    }

    if (url === "/api/settings/general") {
      return jsonResponse({
        error: false,
        data: {
          settings: generalSettings(),
        },
      });
    }

    if (url === "/api/settings/playback") {
      return jsonResponse({
        error: false,
        data: {
          settings: playbackSettings(),
        },
      });
    }

    if (url === "/api/settings") {
      return jsonResponse({
        error: false,
        data: librarySettings(),
      });
    }

    return jsonResponse(
      {
        error: true,
        message: `Unexpected request: ${url}`,
      },
      500,
    );
  });

  vi.stubGlobal("fetch", fetchMock);
}

function createSettingsQueryClient() {
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

async function renderSettingsRoute(
  initialEntry: string,
  playerState: AudioPlayerState | null = null,
) {
  vi.stubGlobal("scrollTo", vi.fn());
  mockSettingsFetch();

  const queryClient = createSettingsQueryClient();
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
      <AudioPlayerStateContext.Provider value={playerState}>
        <RouterProvider router={router} context={{ queryClient }} />
      </AudioPlayerStateContext.Provider>
    </QueryClientProvider>,
  );

  return { queryClient, router };
}

afterEach(() => {
  vi.clearAllTimers();
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  Object.defineProperty(window, "matchMedia", {
    writable: true,
    value: defaultMatchMedia,
  });

  if (originalStartViewTransition === undefined) {
    Reflect.deleteProperty(document, "startViewTransition");
  } else {
    Object.defineProperty(document, "startViewTransition", {
      configurable: true,
      writable: true,
      value: originalStartViewTransition,
    });
  }
});

describe("settings route tab transitions", () => {
  it("delays tab route changes without using native view transitions", async () => {
    const user = userEvent.setup();
    const setTimeoutSpy = vi.spyOn(window, "setTimeout");
    const startViewTransition = vi.fn((callback: () => void) => {
      callback();
      return {
        finished: Promise.resolve(),
        ready: Promise.resolve(),
        updateCallbackDone: Promise.resolve(),
        skipTransition: vi.fn(),
      };
    });

    Object.defineProperty(document, "startViewTransition", {
      configurable: true,
      writable: true,
      value: startViewTransition,
    });

    await renderSettingsRoute("/settings");

    expect(await screen.findByText("Application Behavior")).toBeInTheDocument();

    // Warm the lazy route chunk so navigation renders without a cold dynamic
    // import racing the waitFor timeout on slow CI runners.
    await import("@/routes/_auth/settings/account.lazy");

    await user.click(screen.getByRole("tab", { name: "Account" }));

    expect(screen.getByText("Application Behavior")).toBeInTheDocument();
    expect(screen.queryByText("Profile Information")).not.toBeInTheDocument();
    expect(startViewTransition).not.toHaveBeenCalled();

    await runContentFadeTransitionTimeout(setTimeoutSpy);

    await waitFor(
      () => {
        expect(screen.getByText("Profile Information")).toBeInTheDocument();
      },
      { timeout: 5000 },
    );
    expect(startViewTransition).not.toHaveBeenCalled();
  }, 10_000);

  it("uses shared motion contracts for settings surfaces and switches", async () => {
    await renderSettingsRoute("/settings");

    const title = await screen.findByText("Application Behavior");
    expect(title.closest('[data-slot="card"]')).toHaveClass(
      ...MOTION_SETTINGS_SURFACE_CLASS.split(" "),
    );

    const savePanel =
      screen.getByText("General settings").parentElement?.parentElement;
    expect(savePanel).toHaveClass(...MOTION_SETTINGS_SURFACE_CLASS.split(" "));
    expect(savePanel).toHaveClass("bottom-4");

    const switchControl = screen.getByRole("switch", { name: "Library watcher" });
    expect(switchControl).toHaveClass(
      ...MOTION_SETTINGS_SURFACE_CLASS.split(" "),
    );
    expect(switchControl.firstElementChild).toHaveClass(
      ...MOTION_CONTROL_THUMB_TRANSFORM_CLASS.split(" "),
    );
  });

  it("keeps sticky actions above the minimized audio player", async () => {
    await renderSettingsRoute("/settings", {
      currentTrack: { id: 1 } as NonNullable<AudioPlayerState["currentTrack"]>,
      tracks: [],
      albumCover: null,
      albumTitle: "",
      musicianName: null,
      isPlaying: true,
      isExpanded: false,
      isKeyboardSuspended: false,
      isShuffleMode: false,
      isPlayAllMode: false,
      trimmedCount: 0,
    });

    const savePanel =
      (await screen.findByText("General settings")).parentElement?.parentElement;
    expect(savePanel).toHaveClass("bottom-28", "sm:bottom-24");
    expect(savePanel).not.toHaveClass("bottom-4");
  });
});

describe("settings form query updates", () => {
  it("updates a clean general settings form when query data changes", async () => {
    const { queryClient } = await renderSettingsRoute("/settings");

    expect(await screen.findByLabelText("Static directory")).toHaveValue(
      "/var/lib/igloo/static",
    );

    await act(async () => {
      queryClient.setQueryData([GENERAL_SETTINGS_KEY], {
        error: false,
        data: {
          settings: {
            ...generalSettings(),
            static_dir: "/srv/igloo/static",
            transcode_dir: "/srv/igloo/transcode",
          },
        },
      });
    });

    await waitFor(() => {
      expect(screen.getByLabelText("Static directory")).toHaveValue(
        "/srv/igloo/static",
      );
    });
  });

  it("preserves dirty general settings edits until reset", async () => {
    const user = userEvent.setup();
    const { queryClient } = await renderSettingsRoute("/settings");

    const staticDirectory = await screen.findByLabelText("Static directory");
    await user.clear(staticDirectory);
    await user.type(staticDirectory, "/draft/static");

    await act(async () => {
      queryClient.setQueryData([GENERAL_SETTINGS_KEY], {
        error: false,
        data: {
          settings: {
            ...generalSettings(),
            static_dir: "/srv/igloo/static",
            transcode_dir: "/srv/igloo/transcode",
          },
        },
      });
    });

    expect(screen.getByLabelText("Static directory")).toHaveValue(
      "/draft/static",
    );

    await user.click(screen.getByRole("button", { name: "Reset" }));

    expect(screen.getByLabelText("Static directory")).toHaveValue(
      "/srv/igloo/static",
    );
  });

  it("updates a clean libraries settings form when query data changes", async () => {
    const { queryClient } = await renderSettingsRoute("/settings/libraries");

    expect(await screen.findByLabelText("Movies library path")).toHaveValue(
      "/media/movies",
    );

    await act(async () => {
      queryClient.setQueryData([SETTINGS_KEY], {
        error: false,
        data: {
          ...librarySettings(),
          movies_dir: "/srv/movies",
          music_dir: "/srv/music",
        },
      });
    });

    await waitFor(() => {
      expect(screen.getByLabelText("Movies library path")).toHaveValue(
        "/srv/movies",
      );
    });
  });

  it("preserves dirty library path edits until reset", async () => {
    const user = userEvent.setup();
    const { queryClient } = await renderSettingsRoute("/settings/libraries");

    const moviesPath = await screen.findByLabelText("Movies library path");
    await user.clear(moviesPath);
    await user.type(moviesPath, "/draft/movies");

    await act(async () => {
      queryClient.setQueryData([SETTINGS_KEY], {
        error: false,
        data: {
          ...librarySettings(),
          movies_dir: "/srv/movies",
          music_dir: "/srv/music",
        },
      });
    });

    expect(screen.getByLabelText("Movies library path")).toHaveValue(
      "/draft/movies",
    );

    await user.click(
      screen.getByRole("button", { name: "Reset library paths" }),
    );

    expect(screen.getByLabelText("Movies library path")).toHaveValue(
      "/srv/movies",
    );
  });

  it("updates a clean playback settings form when query data changes", async () => {
    const { queryClient } = await renderSettingsRoute("/settings/playback");

    expect(await screen.findByLabelText("Download speed (Mbps)")).toHaveValue(
      null,
    );

    await act(async () => {
      queryClient.setQueryData([PLAYBACK_SETTINGS_KEY, 1], {
        error: false,
        data: {
          settings: {
            ...playbackSettings(),
            download_mbps: 22.5,
            server_upload_mbps: 25,
          },
        },
      });
    });

    await waitFor(() => {
      expect(screen.getByLabelText("Download speed (Mbps)")).toHaveValue(22.5);
    });
  });

  it("preserves dirty playback settings edits until reset", async () => {
    const user = userEvent.setup();
    const { queryClient } = await renderSettingsRoute("/settings/playback");

    const downloadSpeed = await screen.findByLabelText("Download speed (Mbps)");
    await user.clear(downloadSpeed);
    await user.type(downloadSpeed, "12.5");

    await act(async () => {
      queryClient.setQueryData([PLAYBACK_SETTINGS_KEY, 1], {
        error: false,
        data: {
          settings: {
            ...playbackSettings(),
            download_mbps: 22.5,
            server_upload_mbps: 25,
          },
        },
      });
    });

    expect(screen.getByLabelText("Download speed (Mbps)")).toHaveValue(12.5);

    await user.click(screen.getByRole("button", { name: "Reset" }));

    expect(screen.getByLabelText("Download speed (Mbps)")).toHaveValue(22.5);
  });
});
