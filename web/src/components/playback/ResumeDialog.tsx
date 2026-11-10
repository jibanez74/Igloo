import { useRef, type RefObject } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Spinner } from "@/components/ui/spinner";
import { formatTimecode } from "@/lib/format";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";

type Props = {
  open: boolean;
  resumeTargetSec: number | null;
  /**
   * Where the page was opened, when that is mid-media: the alternative to
   * resuming is then playing from there (keeping the saved progress) rather
   * than starting over.
   */
  playFromSec?: number | null;
  pending: boolean;
  onResume: () => void;
  onStartFromBeginning: () => void;
  onPlayFrom?: () => void;
  restoreFocusRef?: RefObject<HTMLElement | null>;
};

export default function ResumeDialog({
  open,
  resumeTargetSec,
  playFromSec = null,
  pending,
  onResume,
  onStartFromBeginning,
  onPlayFrom,
  restoreFocusRef,
}: Props) {
  const resumeButtonRef = useRef<HTMLButtonElement | null>(null);
  const playFrom =
    playFromSec !== null && playFromSec > 0 && onPlayFrom
      ? { label: formatTimecode(playFromSec), select: onPlayFrom }
      : null;

  let description = "Resume your saved progress or start from the beginning.";
  if (resumeTargetSec !== null && playFrom) {
    description = `Resume from ${formatTimecode(resumeTargetSec)} or play from ${playFrom.label}.`;
  } else if (resumeTargetSec !== null) {
    description = `Resume from ${formatTimecode(resumeTargetSec)} or start from the beginning.`;
  }

  let secondaryLabel = "Start from beginning";
  if (playFrom) {
    secondaryLabel = `Play from ${playFrom.label}`;
  } else if (pending) {
    secondaryLabel = "Clearing progress...";
  }

  return (
    <Dialog open={open}>
      <DialogContent
        showCloseButton={false}
        size="lg"
        elevation="floating"
        onOpenAutoFocus={event => {
          event.preventDefault();
          resumeButtonRef.current?.focus({ preventScroll: true });
        }}
        onCloseAutoFocus={
          restoreFocusRef
            ? event => {
                event.preventDefault();
                focusDialogRestoreTarget(restoreFocusRef.current);
              }
            : undefined
        }
        onEscapeKeyDown={event => event.preventDefault()}
        onPointerDownOutside={event => event.preventDefault()}
        onInteractOutside={event => event.preventDefault()}
      >
        <DialogHeader>
          <DialogTitle>Resume playback?</DialogTitle>
          <DialogDescription>
            {description}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button
            type="button"
            variant="secondary"
            onClick={() => {
              if (playFrom) {
                playFrom.select();
                return;
              }
              onStartFromBeginning();
            }}
            disabled={pending}
          >
            {pending && !playFrom ? (
              <Spinner className="size-4" aria-hidden="true" />
            ) : null}
            {secondaryLabel}
          </Button>
          <Button
            ref={resumeButtonRef}
            type="button"
            variant="accent"
            onClick={() => {
              onResume();
            }}
            disabled={pending}
          >
            Resume
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
