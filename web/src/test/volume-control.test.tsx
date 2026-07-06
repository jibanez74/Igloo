import { type RefObject } from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import VolumeControl from "@/components/VolumeControl";

function renderVolumeControl() {
  const media = document.createElement("audio");
  const mediaRef = { current: media } satisfies RefObject<HTMLAudioElement | null>;

  render(
    <>
      <button type="button">Outside control</button>
      <VolumeControl mediaRef={mediaRef} />
    </>,
  );

  return { media };
}

describe("VolumeControl", () => {
  it("opens the minimized panel as a non-modal control group", async () => {
    const user = userEvent.setup();
    renderVolumeControl();

    const trigger = screen.getByRole("button", { name: "Adjust volume" });
    expect(trigger).not.toHaveAttribute("aria-haspopup");

    await user.click(trigger);

    expect(trigger).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("group", { name: "Volume controls" })).toBeVisible();
    expect(
      screen.queryByRole("dialog", { name: "Volume controls" }),
    ).not.toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Mute" })).toHaveFocus();
    });
  });

  it("closes the minimized panel on Escape and restores trigger focus", async () => {
    const user = userEvent.setup();
    renderVolumeControl();

    const trigger = screen.getByRole("button", { name: "Adjust volume" });
    await user.click(trigger);
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Mute" })).toHaveFocus();
    });

    await user.keyboard("{Escape}");

    await waitFor(() => {
      expect(
        screen.queryByRole("group", { name: "Volume controls" }),
      ).not.toBeInTheDocument();
    });
    expect(trigger).toHaveAttribute("aria-expanded", "false");
    expect(trigger).toHaveFocus();
  });

  it("closes the minimized panel when focus leaves the control", async () => {
    const user = userEvent.setup();
    renderVolumeControl();

    await user.click(screen.getByRole("button", { name: "Adjust volume" }));
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Mute" })).toHaveFocus();
    });

    const outsideButton = screen.getByRole("button", { name: "Outside control" });
    outsideButton.focus();
    fireEvent.focusIn(outsideButton);

    await waitFor(() => {
      expect(
        screen.queryByRole("group", { name: "Volume controls" }),
      ).not.toBeInTheDocument();
    });
  });

  it("syncs volume and mute state from media volumechange events", async () => {
    const user = userEvent.setup();
    const { media } = renderVolumeControl();

    media.volume = 0.42;
    media.muted = true;
    fireEvent(media, new Event("volumechange"));

    await user.click(screen.getByRole("button", { name: "Adjust volume" }));

    expect(screen.getByRole("button", { name: "Unmute" })).toBeVisible();
    expect(screen.getByRole("slider", { name: "Volume" })).toHaveAttribute(
      "aria-valuetext",
      "0% volume",
    );
    expect(screen.getByText("0%")).toBeVisible();
  });

  it("unmutes media with a single grouped state update when the slider changes", async () => {
    const user = userEvent.setup();
    const { media } = renderVolumeControl();

    media.volume = 0.4;
    media.muted = true;
    fireEvent(media, new Event("volumechange"));

    await user.click(screen.getByRole("button", { name: "Adjust volume" }));

    const slider = screen.getByRole("slider", { name: "Volume" });
    fireEvent.change(slider, { target: { value: "0.25" } });

    expect(media.volume).toBe(0.25);
    expect(media.muted).toBe(false);
    expect(slider).toHaveAttribute("aria-valuetext", "25% volume");
    expect(screen.getByRole("button", { name: "Mute" })).toBeVisible();
  });
});
