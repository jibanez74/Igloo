import { useRef, useState } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import ConfirmDialog from "@/components/shared/ConfirmDialog";

function ConfirmDialogHarness({ onConfirm }: { onConfirm: () => void }) {
  const [open, setOpen] = useState(true);
  const openerRef = useRef<HTMLButtonElement | null>(null);

  return (
    <>
      <button ref={openerRef} type="button">
        Open menu
      </button>
      <ConfirmDialog
        open={open}
        onOpenChange={setOpen}
        title="Delete item"
        description="This cannot be undone."
        confirmLabel="Delete"
        restoreFocusRef={openerRef}
        onConfirm={onConfirm}
      />
    </>
  );
}

describe("ConfirmDialog", () => {
  it("does not close automatically when confirm is clicked", async () => {
    const onConfirm = vi.fn();
    const user = userEvent.setup();

    render(<ConfirmDialogHarness onConfirm={onConfirm} />);

    await user.click(screen.getByRole("button", { name: "Delete" }));

    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(
      screen.getByRole("alertdialog", { name: "Delete item" }),
    ).toBeInTheDocument();
  });

  it("restores focus to the supplied opener when canceled", async () => {
    const user = userEvent.setup();

    render(<ConfirmDialogHarness onConfirm={vi.fn()} />);

    await user.click(screen.getByRole("button", { name: "Cancel" }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Open menu" })).toHaveFocus();
    });
  });

  it("disables actions while pending", () => {
    render(
      <ConfirmDialog
        open
        onOpenChange={vi.fn()}
        title="Delete item"
        confirmLabel="Delete"
        pending
        onConfirm={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: "Cancel" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Delete..." })).toBeDisabled();
  });
});
