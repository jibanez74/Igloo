import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import SettingsSaveBar from "@/components/settings/SettingsSaveBar";

function renderBar(
  props: Partial<React.ComponentProps<typeof SettingsSaveBar>> = {},
) {
  render(
    <SettingsSaveBar
      title="General settings"
      isDirty={false}
      statusMessage="Saved settings are used on future requests."
      onReset={vi.fn()}
      isPending={false}
      stickyClassName="bottom-4"
      {...props}
    />,
  );
  return screen.getByText("General settings").parentElement!.parentElement!;
}

describe("SettingsSaveBar", () => {
  it("rests in flow with disabled actions while the form is clean", () => {
    const bar = renderBar();

    expect(bar).toHaveClass("bg-card");
    expect(bar).not.toHaveClass("sticky", "bottom-4");
    expect(bar).toHaveTextContent("No unsaved changes");
    expect(screen.getByRole("button", { name: "Reset" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Save Settings" })).toBeDisabled();
  });

  it("sticks at the given offset with live actions while dirty", () => {
    const bar = renderBar({ isDirty: true });

    expect(bar).toHaveClass("sticky", "z-10", "bottom-4");
    expect(bar).toHaveTextContent("Saved settings are used on future requests.");
    expect(screen.getByRole("button", { name: "Reset" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Save Settings" })).toBeEnabled();
  });

  // Libraries confirms a save through the bar's own status line; a clean bar
  // must not swallow "Library paths saved." with its idle text.
  it("keeps a success or error message when the form is clean", () => {
    renderBar({ statusMessage: "Library paths saved.", statusTone: "success" });

    expect(screen.getByText("Library paths saved.")).toHaveClass("text-success");
    expect(screen.queryByText("No unsaved changes")).toBeNull();
  });

  it("never sticks without an offset, and drops its own chrome when embedded", () => {
    const bar = renderBar({ isDirty: true, stickyClassName: undefined, embedded: true });

    expect(bar).not.toHaveClass("sticky", "shadow-lg", "rounded-lg");
    expect(bar).toHaveClass("border-t");
  });

  it("keeps the Reset name while its label is icon-only on phones", () => {
    renderBar({ isDirty: true, resetLabel: "Reset library paths" });

    const reset = screen.getByRole("button", { name: "Reset library paths" });
    expect(reset.querySelector("span")).toHaveClass("sr-only", "sm:not-sr-only");
  });

  it("disables both actions while a save is pending", () => {
    renderBar({ isDirty: true, isPending: true });

    expect(screen.getByRole("button", { name: "Reset" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Saving..." })).toBeDisabled();
  });
});
