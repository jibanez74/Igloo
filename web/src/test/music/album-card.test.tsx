import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import AlbumCard from "@/components/music/AlbumCard";
import { albumDetailsQueryOpts } from "@/lib/query-opts";
import { audioPlayerActionsMock } from "../helpers/audio-player";
import { nullableString } from "../helpers/fixtures";
import { simpleAlbum } from "../helpers/music";
import { createTestQueryClient, renderWithQueryClient } from "../helpers/render";

const showActionFailedMock = vi.fn();

vi.mock("@tanstack/react-router", async () =>
  (await import("../helpers/router-link-mock")).routerWithAnchorLinks(),
);
vi.mock("@/hooks/useAudioPlayerActions", () => import("../helpers/audio-player"));
vi.mock("@/lib/toast-helpers", async () => ({
  ...(await vi.importActual<typeof import("@/lib/toast-helpers")>(
    "@/lib/toast-helpers",
  )),
  showActionFailed: (...args: unknown[]) => showActionFailedMock(...args),
}));

describe("AlbumCard", () => {
  it("reports an album that fails to load instead of doing nothing on Play", async () => {
    const user = userEvent.setup();
    const queryClient = createTestQueryClient();
    const failure = { error: true as const, message: "album query failed", status: 500 };
    queryClient.setQueryData(albumDetailsQueryOpts(42).queryKey, failure);

    renderWithQueryClient(
      <AlbumCard
        album={simpleAlbum({
          id: 42,
          title: "Blue Record",
          musician: nullableString("The Band"),
        })}
      />,
      { queryClient },
    );

    await user.click(
      screen.getByRole("button", { name: "Play Blue Record by The Band" }),
    );

    await waitFor(() => {
      expect(showActionFailedMock).toHaveBeenCalledWith("play album", failure);
    });
    expect(audioPlayerActionsMock.playQueue).not.toHaveBeenCalled();
  });
});
