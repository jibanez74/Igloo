import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import SettingsSaveBar from "@/components/settings/SettingsSaveBar";
import { expectNoClass } from "../helpers/dom";

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
    expectNoClass(bar, "sticky", "bottom-4");
    expect(bar).toHaveTextContent("No unsaved changes");
    expect(screen.getByRole("button", { name: "Reset" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Save settings" })).toBeDisabled();
  });

  it("sticks at the given offset with live actions while dirty", () => {
    const bar = renderBar({ isDirty: true });

    expect(bar).toHaveClass("sticky", "z-10", "bottom-4");
    expect(bar).toHaveTextContent("Saved settings are used on future requests.");
    expect(screen.getByRole("button", { name: "Reset" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Save settings" })).toBeEnabled();
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

    expectNoClass(bar, "sticky", "shadow-lg", "rounded-lg");
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

  // Save and Reset disable themselves once there is nothing to do. A disabled
  // button cannot hold focus, so the browser dropped it on the page body and
  // the next Tab started from the top (seen in Chromium and Firefox).
  describe("focus when its buttons disable themselves", () => {
    function Form({ fails = false }: { fails?: boolean }) {
      const [dirty, setDirty] = useState(true);
      const [pending, setPending] = useState(false);
      return (
        <form
          onSubmit={event => {
            event.preventDefault();
            setPending(true);
          }}
        >
          <SettingsSaveBar
            title="General settings"
            isDirty={dirty}
            statusMessage="Saved settings are used on future requests."
            onReset={() => setDirty(false)}
            isPending={pending}
          />
          <button
            type="button"
            onClick={() => {
              setPending(false);
              if (!fails) setDirty(false);
            }}
          >
            finish the save
          </button>
        </form>
      );
    }

    const statusLine = () =>
      screen.getByText(
        /No unsaved changes|Saved settings are used on future requests\./,
      );

    it("moves to the status line when a save lands", async () => {
      const user = userEvent.setup();
      render(<Form />);

      const save = screen.getByRole("button", { name: "Save settings" });
      save.focus();
      await user.keyboard("{Enter}");
      expect(statusLine()).toHaveFocus();

      // Finish the save without moving focus, as a response would.
      screen.getByRole("button", { name: "finish the save" }).click();
      expect(await screen.findByText("No unsaved changes")).toHaveFocus();
      expect(screen.getByRole("button", { name: "Save settings" })).toBeDisabled();
    });

    it("gives focus back to Save when the save fails", async () => {
      const user = userEvent.setup();
      render(<Form fails />);

      const save = screen.getByRole("button", { name: "Save settings" });
      save.focus();
      await user.keyboard("{Enter}");
      expect(statusLine()).toHaveFocus();

      screen.getByRole("button", { name: "finish the save" }).click();
      await screen.findByRole("button", { name: "Save settings" });
      await vi.waitFor(() => expect(save).toHaveFocus());
      expect(save).toBeEnabled();
    });

    it("moves to the status line after Reset", async () => {
      const user = userEvent.setup();
      render(<Form />);

      await user.click(screen.getByRole("button", { name: "Reset" }));
      expect(screen.getByText("No unsaved changes")).toHaveFocus();
    });

    it("leaves focus alone when it is somewhere else", async () => {
      const user = userEvent.setup();
      render(<Form />);

      const other = screen.getByRole("button", { name: "finish the save" });
      await user.click(other);
      expect(other).toHaveFocus();
    });
  });
});
