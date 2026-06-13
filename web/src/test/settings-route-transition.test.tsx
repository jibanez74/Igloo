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
  MOTION_CONTROL_THUMB_TRANSFORM_CLASS,
  MOTION_SETTINGS_SURFACE_CLASS,
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
