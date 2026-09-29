import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import PlaylistFormDialog from "@/components/music/PlaylistFormDialog";
import { PLAYLIST_DETAILS_KEY } from "@/lib/constants";
import { createTestQueryClient, renderWithQueryClient } from "../helpers/render";
import {
  expectRefetchedBeforeReload,
  recordRefreshOrder,
} from "../helpers/route-refresh";

const apiMocks = vi.hoisted(() => ({
  updatePlaylist: vi.fn(),
}));
const routerMocks = vi.hoisted(() => ({
  invalidate: vi.fn(() => Promise.resolve()),
}));

vi.mock("@/lib/api", async () => {
  const actual =
    await vi.importActual<typeof import("@/lib/api")>("@/lib/api");

  return {
    ...actual,
    updatePlaylist: (...args: unknown[]) => apiMocks.updatePlaylist(...args),
  };
});

// A rename reloads the playlist route so its head picks up the new name; the
// tests render without a router.
vi.mock("@tanstack/react-router", async () => {
  const actual = await vi.importActual<
    typeof import("@tanstack/react-router")
  >("@tanstack/react-router");

  return { ...actual, useRouter: () => routerMocks };
});

describe("PlaylistFormDialog", () => {
  it("gives the description textarea an accessible name from the visible label", () => {
    const queryClient = createTestQueryClient();

    renderWithQueryClient(
      <PlaylistFormDialog mode="create" open onOpenChange={vi.fn()} />,
      { queryClient },
    );

    expect(
      screen.getByRole("textbox", { name: /^Description/ }),
    ).toHaveAccessibleName("Description (optional)");
  });

  it("refetches the playlist and reloads its route after a rename", async () => {
    const user = userEvent.setup();
    const queryClient = createTestQueryClient();
    const refreshOrder = recordRefreshOrder(queryClient, routerMocks.invalidate);
    const onOpenChange = vi.fn();
    apiMocks.updatePlaylist.mockResolvedValue({
      error: false,
      data: { playlist: { id: 7 } },
    });

    renderWithQueryClient(
      <PlaylistFormDialog
        mode="edit"
        open
        onOpenChange={onOpenChange}
        playlist={{
          id: 7,
          name: "Road Trip",
          description: { String: "", Valid: false },
          is_public: false,
        }}
      />,
      { queryClient },
    );

    const nameInput = screen.getByRole("textbox", { name: /^Name/ });
    await user.clear(nameInput);
    await user.type(nameInput, "Night Drive");
    await user.click(screen.getByRole("button", { name: "Save Changes" }));

    await waitFor(() => {
      expect(routerMocks.invalidate).toHaveBeenCalledOnce();
    });
    expect(apiMocks.updatePlaylist).toHaveBeenCalledWith(7, {
      name: "Night Drive",
      description: undefined,
      is_public: false,
    });
    expect(onOpenChange).toHaveBeenCalledWith(false);
    expectRefetchedBeforeReload(refreshOrder, [PLAYLIST_DETAILS_KEY, 7]);
  });
});
