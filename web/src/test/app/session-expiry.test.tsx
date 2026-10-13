import { act, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { AudioPlayerActionsContext } from "@/context/AudioPlayerContext";
import { getNotifications } from "@/lib/api";
import type { AudioPlayerActions } from "@/types/audio-player";
import { jsonResponse, requestURL } from "../helpers/api";
import { authUser } from "../helpers/fixtures";
import { renderRoute } from "../helpers/render-route";

const toastMocks = vi.hoisted(() => ({ showInfo: vi.fn() }));

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
      return requestURL(input) === "/api/auth/user"
        ? jsonResponse(authUser())
        : jsonResponse({ error: false, data: {} });
    }),
  );

  return server;
}

function renderShell(path = "/photos") {
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
    await screen.findByRole("button", { name: "Logout" });

    // The server ends the session; the next refetch (a window focus, say)
    // is the first request to hear about it.
    server.auth = "unauthorized";
    await act(async () => {
      await queryClient.refetchQueries({ type: "active" });
    });

    await waitFor(() => {
      expect(router.state.location.pathname).toBe("/login");
    });
    expect(router.state.location.search).toEqual({ redirect: "/photos" });
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
    await screen.findByRole("button", { name: "Logout" });

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
      expect(router.state.location.pathname).toBe("/photos");

      server.auth = "user";
      await user.click(screen.getByRole("button", { name: "Try again" }));

      expect(await screen.findByRole("button", { name: "Logout" })).toBeInTheDocument();
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
    expect(router.state.location.search).toEqual({ redirect: "/photos" });
    // Nobody was signed in, so there is no sign-out to announce.
    expect(toastMocks.showInfo).not.toHaveBeenCalled();
  });
});
