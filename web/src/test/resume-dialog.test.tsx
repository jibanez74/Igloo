import { useRef, useState } from "react";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import ResumeDialog from "@/components/ResumeDialog";

function ResumeDialogHarness({
  onResume,
  onStartFromBeginning,
}: {
  onResume: () => void;
  onStartFromBeginning: () => void;
}) {
  const [open, setOpen] = useState(true);
  const playerRef = useRef<HTMLDivElement | null>(null);

  return (
    <>
      <div ref={playerRef} role="region" aria-label="Video player" tabIndex={-1} />
      <ResumeDialog
        open={open}
        savedProgressSec={125}
        pending={false}
        restoreFocusRef={playerRef}
        onResume={() => {
          onResume();
          setOpen(false);
        }}
        onStartFromBeginning={() => {
          onStartFromBeginning();
          setOpen(false);
        }}
      />
    </>
  );
}

describe("ResumeDialog", () => {
  it("starts focus on Resume and restores focus to the player after choosing", async () => {
    const onResume = vi.fn();
    const onStartFromBeginning = vi.fn();
    const user = userEvent.setup();

    render(
      <ResumeDialogHarness
        onResume={onResume}
        onStartFromBeginning={onStartFromBeginning}
      />,
    );

    const resumeButton = screen.getByRole("button", { name: "Resume" });

    await waitFor(() => {
      expect(resumeButton).toHaveFocus();
    });

    await user.click(resumeButton);

    expect(onResume).toHaveBeenCalledTimes(1);
    expect(onStartFromBeginning).not.toHaveBeenCalled();

    await waitFor(() => {
      expect(screen.getByRole("region", { name: "Video player" })).toHaveFocus();
    });
  });

  it("disables both choices while an action is pending", () => {
    render(
      <ResumeDialog
        open
        savedProgressSec={125}
        pending
        onResume={vi.fn()}
        onStartFromBeginning={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: "Resume" })).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Clearing progress..." }),
    ).toBeDisabled();
  });
});
