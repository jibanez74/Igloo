import type { ReactNode } from "react";
import type { QueryClient } from "@tanstack/react-query";
import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { AudioPlayerActionsContext } from "@/context/audio-player-contexts";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { getNotifications } from "@/lib/api";
import { authUserQueryOpts } from "@/lib/query-opts";
import { beginEndingSession, forgetEndedSession } from "@/lib/session";
import type { AudioPlayerActions } from "@/types/audio-player";
import { deferredResponse, jsonResponse, requestURL } from "../helpers/api";
import { stubMediaElement } from "../helpers/dom";
import { authUser } from "../helpers/fixtures";
import { homeShellResponse } from "../helpers/home";
import { renderRoute } from "../helpers/render-route";

const toastMocks = vi.hoisted(() => ({ showInfo: vi.fn() }));

// The shell mounts the real player, which would shadow the player each test
// puts above the router: a stub whose stop() it watches, or (for the pending
// Shuffle all) the real provider.
vi.mock("@/context/AudioPlayerContext", () => ({
  AudioPlayerProvider: ({ children }: { children: ReactNode }) => children,
}));
const { AudioPlayerProvider } = await vi.importActual<
  typeof import("@/context/AudioPlayerContext")
>("@/context/AudioPlayerContext");

vi.mock("@/lib/toast-helpers", async () => ({
  ...(await vi.importActual<typeof import("@/lib/toast-helpers")>(
    "@/lib/toast-helpers",
  )),
  showInfo: toastMocks.showInfo,
}));

type AuthAnswer = "user" | "unauthorized" | "server-error" | "offline";

// Every request answers by the session's current state, so a test can end
// the session (or take the server away) between two steps.
function mockServer(initial: AuthAnswer) {
  const server = { auth: initial };

  vi.stubGlobal(
    "fetch",
    vi.fn((input: RequestInfo | URL) => {
      if (server.auth === "offline") {
        return Promise.reject(new TypeError("Failed to fetch"));
      }
      if (server.auth === "server-error") {
        return jsonResponse({ error: true, message: "internal error" }, 500);
      }
      if (server.auth === "unauthorized") {
        return jsonResponse({ error: true, message: "not authorized" }, 401);
      }
      return homeShellResponse(input);
    }),
  );

  return server;
}

function renderShell(path = "/") {
  const stop = vi.fn();
  const view = renderRoute(path, {
    wrapper: children => (
      <AudioPlayerActionsContext.Provider
        value={{ stop } as unknown as AudioPlayerActions}
      >
        {children}
      </AudioPlayerActionsContext.Provider>
    ),
  });
  return { stop, view };
}

describe("a session that ends while the shell is open", () => {
  it("signs out and offers the page back after signing in again", async () => {
    const server = mockServer("user");
    const { stop, view } = renderShell();
    const { router, queryClient } = await view;
    await screen.findByRole("button", { name: /^Account: / });

    // The server ends the session; the next refetch (a window focus, say)
    // is the first request to hear about it.
    server.auth = "unauthorized";
    await act(async () => {
      await queryClient.refetchQueries({ type: "active" });
    });

    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/login");
    });
    expect(router.state.location.search).toEqual({ redirect: "/" });
    expect(router.state.location.state).toMatchObject({ loggedOut: true });
    expect(stop).toHaveBeenCalledOnce();
    expect(toastMocks.showInfo).toHaveBeenCalledExactlyOnceWith("You've been signed out.");
    // Nothing the ended session loaded stays cached.
    await waitFor(() => {
      expect(queryClient.getQueryCache().getAll()).toEqual([]);
    });
  });

  it("signs out once however many requests hear about it", async () => {
    const server = mockServer("user");
    const { stop, view } = renderShell();
    const { router } = await view;
    await screen.findByRole("button", { name: /^Account: / });

    server.auth = "unauthorized";
    await act(async () => {
      await Promise.all([getNotifications(), getNotifications(), getNotifications()]);
    });

    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/login");
    });
    expect(stop).toHaveBeenCalledOnce();
    expect(toastMocks.showInfo).toHaveBeenCalledOnce();
  });
});

// Ending the session stops the player, and a "Shuffle all" still waiting for
// its first page must not bring the player back once that page lands. The
// shell's own player goes when the shell does; this one is mounted above the
// router, so it outlives the sign-out and only stop()'s fence keeps it quiet.
describe("a pending Shuffle all when the session ends", () => {
  const actions: { current?: AudioPlayerActions } = {};

  function ActionsProbe() {
    actions.current = useAudioPlayerActions();
    return null;
  }

  it("does not recreate the player on the login page", async () => {
    stubMediaElement();
    const shuffle = deferredResponse();
    const server = { auth: "user" as AuthAnswer };
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) => {
        if (requestURL(input).startsWith("/api/music/tracks/shuffle")) {
          return shuffle.promise;
        }
        if (server.auth === "unauthorized") {
          return jsonResponse({ error: true, message: "not authorized" }, 401);
        }
        return homeShellResponse(input);
      }),
    );

    const { router, queryClient } = await renderRoute("/", {
      wrapper: children => (
        <AudioPlayerProvider>
          <ActionsProbe />
          {children}
        </AudioPlayerProvider>
      ),
    });
    await screen.findByRole("button", { name: /^Account: / });

    let started: Promise<void> | undefined;
    act(() => {
      started = actions.current?.startShufflePlayback();
    });

    server.auth = "unauthorized";
    await act(async () => {
      await queryClient.refetchQueries({ type: "active" });
    });
    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/login");
    });
    await waitFor(() => {
      expect(queryClient.getQueryCache().getAll()).toEqual([]);
    });

    shuffle.resolve(
      jsonResponse({
        error: false,
        data: {
          tracks: [
            {
              id: 1,
              title: "Previous session's song",
              duration: 100,
              codec: "flac",
              bit_rate: 900000,
              album_id: { Int64: 1, Valid: true },
              album_title: { String: "", Valid: false },
              album_cover: { String: "", Valid: false },
              musician_id: { Int64: 1, Valid: true },
              musician_name: { String: "The Band", Valid: true },
            },
          ],
        },
      }),
    );
    await act(async () => {
      await started;
    });

    expect(document.querySelector("audio")).toBeNull();
    expect(screen.queryByRole("dialog", { name: /Now playing/i })).toBeNull();
    expect(screen.queryByText("Previous session's song")).toBeNull();
  });
});

describe("the shell's auth check", () => {
  it.each([
    ["the server is unreachable", "offline"],
    ["the server fails", "server-error"],
  ] as const)(
    "shows a load error with Try again, not the login page, when %s",
    async (_name, failure) => {
      const server = mockServer(failure);
      const user = userEvent.setup();
      const { view } = renderShell();
      const { router } = await view;

      expect(await screen.findByRole("alert")).toHaveTextContent(
        "Couldn’t load Igloo.",
      );
      expect(screen.getByRole("heading", { level: 1, name: "Igloo" })).toBeInTheDocument();
      expect(router.state.location.pathname).toBe("/");

      server.auth = "user";
      await user.click(screen.getByRole("button", { name: "Try again" }));

      expect(await screen.findByRole("button", { name: /^Account: / })).toBeInTheDocument();
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    },
  );

  it("still sends a signed-out visitor to log in, remembering the page", async () => {
    mockServer("unauthorized");
    const { view } = renderShell();
    const { router } = await view;

    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/login");
    });
    expect(router.state.location.search).toEqual({ redirect: "/" });
    // Nobody was signed in, so there is no sign-out to announce.
    expect(toastMocks.showInfo).not.toHaveBeenCalled();
  });
});

// The shell's guard runs before every navigation and hover preload under
// /_auth. A signed-in user checked within the last minute is answered from
// the cache, so those navigations do not wait on the session.
describe("the shell's guard once someone is signed in", () => {
  function countingServer() {
    const server = {
      authRequests: 0,
      auth: undefined as ReturnType<typeof deferredResponse> | undefined,
      unauthorized: false,
    };
    vi.stubGlobal(
      "fetch",
      vi.fn((input: RequestInfo | URL) => {
        if (requestURL(input) === "/api/auth/user") {
          server.authRequests += 1;
          if (server.unauthorized) {
            return jsonResponse({ error: true, message: "not authorized" }, 401);
          }
          if (server.auth) return server.auth.promise;
        }
        return homeShellResponse(input);
      }),
    );
    return server;
  }

  function ageAuthEntry(queryClient: QueryClient) {
    queryClient
      .getQueryCache()
      .find({ queryKey: authUserQueryOpts().queryKey, exact: true })
      ?.setState({ dataUpdatedAt: Date.now() - 2 * 60_000 });
  }

  it("does not ask the server again while the user is fresh", async () => {
    const server = countingServer();
    const { router } = await renderShell().view;
    await screen.findByRole("button", { name: /^Account: / });
    expect(server.authRequests).toBe(1);

    await act(async () => {
      await router.invalidate();
      await router.preloadRoute({ to: "/" });
    });

    expect(server.authRequests).toBe(1);
  });

  it("waits for a fresh check once the user is a minute old", async () => {
    const server = countingServer();
    const { router, queryClient } = await renderShell().view;
    await screen.findByRole("button", { name: /^Account: / });

    ageAuthEntry(queryClient);
    server.auth = deferredResponse();
    let settled = false;
    let navigation: Promise<void> | undefined;
    act(() => {
      navigation = router.invalidate().then(() => {
        settled = true;
      });
    });
    await waitFor(() => {
      expect(server.authRequests).toBe(2);
    });
    expect(settled).toBe(false);

    server.auth.resolve(jsonResponse(authUser()));
    await act(async () => {
      await navigation;
    });
    expect(settled).toBe(true);
    expect(router.state.location.pathname).toBe("/");
  });

  it("checks again, rather than reuse, a session check that failed", async () => {
    const server = countingServer();
    const { router, queryClient } = await renderShell().view;
    await screen.findByRole("button", { name: /^Account: / });

    queryClient.setQueryData(authUserQueryOpts().queryKey, {
      error: true,
      message: "500 - internal error",
    });
    await act(async () => {
      await router.invalidate();
    });

    expect(server.authRequests).toBe(2);
    expect(router.state.location.pathname).toBe("/");
  });

  it("asks the server, rather than reuse a fresh user, while the session is ending, and leaves on its 401", async () => {
    const server = countingServer();
    const { router } = await renderShell().view;
    await screen.findByRole("button", { name: /^Account: / });
    expect(server.authRequests).toBe(1);

    beginEndingSession();
    server.unauthorized = true;
    try {
      await act(async () => {
        await router.invalidate();
      });
      expect(server.authRequests).toBe(2);
      await waitFor(() => {
        expect(router.state.location.pathname).toBe("/login");
      });
    } finally {
      forgetEndedSession();
    }
  });

  it("signs out when the check finds the session gone", async () => {
    const server = countingServer();
    const { router, queryClient } = await renderShell().view;
    await screen.findByRole("button", { name: /^Account: / });

    ageAuthEntry(queryClient);
    server.unauthorized = true;
    await act(async () => {
      await router.invalidate();
    });

    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/login");
    });
    expect(toastMocks.showInfo).toHaveBeenCalledExactlyOnceWith("You've been signed out.");
  });
});
