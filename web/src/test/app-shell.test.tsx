import type React from "react";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import AppShell from "@/components/AppShell";
import { AudioPlayerProvider } from "@/context/AudioPlayerContext";
import { useAudioPlayerActions } from "@/hooks/useAudioPlayerActions";
import { convertToAudioTrack } from "@/lib/audio-utils";

vi.mock("@/components/app-sidebar", () => ({
  default: () => (
    <nav aria-label="Main navigation">
      <a href="/">Home</a>
    </nav>
  ),
}));

vi.mock("@/components/Header", () => ({
  default: () => <div role="search" aria-label="Search library" />,
}));

vi.mock("@/components/ui/sidebar", () => ({
  SidebarInset: ({
    children,
    ...props
  }: React.ComponentProps<"main">) => <main {...props}>{children}</main>,
  SidebarProvider: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SidebarTrigger: (props: React.ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button type="button" aria-label="Toggle Sidebar" {...props} />
  ),
}));

function PlayerHarness() {
  const actions = useAudioPlayerActions();

  const startPlayback = () => {
    const testTrack = convertToAudioTrack({
      id: 1,
      title: "Test Track",
      file_path: "/music/test.flac",
      duration: 100,
      codec: "flac",
      bit_rate: 1,
      album_id: { Int64: 0, Valid: false },
      musician_id: { Int64: 0, Valid: false },
      album_cover: { String: "", Valid: false },
      musician_name: { String: "", Valid: false },
    });

    actions.playTrack(testTrack, [testTrack], {
      cover: null,
      title: "Test Album",
      musician: null,
    });
  };

  return (
    <>
      <button type="button" onClick={startPlayback}>
        start playback
      </button>
      <button type="button" onClick={actions.minimize}>
        minimize player
      </button>
      <button type="button" onClick={actions.stop}>
        stop playback
      </button>
    </>
  );
}

describe("AppShell", () => {
  it("renders one protected app shell around route content", () => {
    render(
      <AppShell>
        <h1>Route content</h1>
      </AppShell>,
    );

    expect(
      screen.getByRole("navigation", { name: /main navigation/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("search", { name: /search library/i }),
    ).toBeInTheDocument();

    const mainLandmarks = screen.getAllByRole("main");
    expect(mainLandmarks).toHaveLength(1);

    const [main] = mainLandmarks;
    expect(main).toHaveAttribute("id", "main");
    expect(
      within(main).getByRole("heading", { name: /route content/i }),
    ).toBeInTheDocument();
  });

  describe("minimized player spacing", () => {
    const originalLoad = HTMLMediaElement.prototype.load;
    const originalPlay = HTMLMediaElement.prototype.play;
    const originalPause = HTMLMediaElement.prototype.pause;

    beforeEach(() => {
      HTMLMediaElement.prototype.load = vi.fn();
      HTMLMediaElement.prototype.play = vi.fn().mockResolvedValue(undefined);
      HTMLMediaElement.prototype.pause = vi.fn();
    });

    afterEach(() => {
      HTMLMediaElement.prototype.load = originalLoad;
      HTMLMediaElement.prototype.play = originalPlay;
      HTMLMediaElement.prototype.pause = originalPause;
    });

    it("reserves bottom space only while the minimized player is visible", () => {
      render(
        <AudioPlayerProvider>
          <AppShell>
            <h1>Route content</h1>
          </AppShell>
          <PlayerHarness />
        </AudioPlayerProvider>,
      );

      const main = screen.getByRole("main");
      const scroller = main.querySelector(".overflow-x-clip");
      if (!scroller) {
        throw new Error("Content container was not rendered");
      }

      expect(scroller).not.toHaveClass("pb-28");

      // Starting playback opens the expanded player: still no reserved space.
      fireEvent.click(screen.getByRole("button", { name: "start playback" }));
      expect(scroller).not.toHaveClass("pb-28");

      // While the modal player dialog is open the harness is aria-hidden, so
      // query by text instead of role.
      fireEvent.click(screen.getByText("minimize player"));
      expect(scroller).toHaveClass("pb-28", "sm:pb-24");

      fireEvent.click(screen.getByRole("button", { name: "stop playback" }));
      expect(scroller).not.toHaveClass("pb-28");
    });
  });
});
