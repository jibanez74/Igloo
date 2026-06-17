import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  RouterProvider,
  createMemoryHistory,
  createRouter,
} from "@tanstack/react-router";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  CONTENT_FADE_TRANSITION_MS,
  GENERAL_SETTINGS_KEY,
  MOTION_CONTROL_THUMB_TRANSFORM_CLASS,
  MOTION_SETTINGS_SURFACE_CLASS,
  PLAYBACK_SETTINGS_KEY,
} from "@/lib/constants";
import { routeTree } from "@/routeTree.gen";

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
    hardware_acceleration_device: "cpu",
    enable_logger: true,
    enable_watcher: false,
    download_images: true,
    static_dir: "/var/lib/igloo/static",
    logs_dir: "/var/lib/igloo/logs",
    transcode_dir: "/var/lib/igloo/transcode",
    server_upload_mbps: null,
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
    is_admin: true,
    preferred_audio_language: null,
    preferred_subtitle_language: null,
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

async function renderSettingsRoute(initialEntry: string) {
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
      <RouterProvider router={router} context={{ queryClient }} />
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

    expect(await screen.findByText("General Settings")).toBeInTheDocument();

    await user.click(screen.getByRole("tab", { name: "Account" }));

    expect(screen.getByText("General Settings")).toBeInTheDocument();
    expect(screen.queryByText("Profile Information")).not.toBeInTheDocument();
    expect(startViewTransition).not.toHaveBeenCalled();

    const transitionCallIndex = setTimeoutSpy.mock.calls.findIndex(
      ([, delay]) => delay === CONTENT_FADE_TRANSITION_MS,
    );

    expect(transitionCallIndex).toBeGreaterThanOrEqual(0);

    await waitFor(() => {
      expect(screen.getByText("Profile Information")).toBeInTheDocument();
    });
    expect(startViewTransition).not.toHaveBeenCalled();
  });

  it("uses shared motion contracts for settings surfaces and switches", async () => {
    await renderSettingsRoute("/settings");

    const title = await screen.findByText("General Settings");
    expect(title.closest('[data-slot="card"]')).toHaveClass(
      ...MOTION_SETTINGS_SURFACE_CLASS.split(" "),
    );

    const savePanel =
      screen.getByText("General settings").parentElement?.parentElement;
    expect(savePanel).toHaveClass(...MOTION_SETTINGS_SURFACE_CLASS.split(" "));

    const switchControl = screen.getByRole("switch", { name: "File logging" });
    expect(switchControl).toHaveClass(
      ...MOTION_SETTINGS_SURFACE_CLASS.split(" "),
    );
    expect(switchControl.firstElementChild).toHaveClass(
      ...MOTION_CONTROL_THUMB_TRANSFORM_CLASS.split(" "),
    );
  });
});

describe("settings form query updates", () => {
  it("resets the general settings form and validation when query data changes", async () => {
    const user = userEvent.setup();
    const { queryClient } = await renderSettingsRoute("/settings");

    const staticDirectory = await screen.findByLabelText("Static directory");
    await user.clear(staticDirectory);
    await user.click(screen.getByRole("button", { name: "Save Settings" }));

    expect(
      await screen.findByText("Static directory is required."),
    ).toBeInTheDocument();
    expect(staticDirectory).toHaveAttribute("aria-invalid", "true");

    await act(async () => {
      queryClient.setQueryData([GENERAL_SETTINGS_KEY], {
        error: false,
        data: {
          settings: {
            ...generalSettings(),
            static_dir: "/srv/igloo/static",
            logs_dir: "/srv/igloo/logs",
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
    expect(
      screen.queryByText("Static directory is required."),
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText("Static directory")).not.toHaveAttribute(
      "aria-invalid",
      "true",
    );
  });

  it("resets the playback settings form and validation when query data changes", async () => {
    const user = userEvent.setup();
    const { queryClient } = await renderSettingsRoute("/settings/playback");

    const downloadSpeed = await screen.findByLabelText("Download speed (Mbps)");
    await user.clear(downloadSpeed);
    await user.type(downloadSpeed, "10000");
    await user.click(screen.getByRole("button", { name: "Save Settings" }));

    expect(
      await screen.findByText(
        "Download speed must be between 0 and 10000 Mbps.",
      ),
    ).toBeInTheDocument();
    expect(downloadSpeed).toHaveAttribute("aria-invalid", "true");

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
    expect(
      screen.queryByText(
        "Download speed must be between 0 and 10000 Mbps.",
      ),
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText("Download speed (Mbps)")).not.toHaveAttribute(
      "aria-invalid",
      "true",
    );
  });
});
