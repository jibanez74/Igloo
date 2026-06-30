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
import { formatTimeSeconds } from "@/lib/format";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import { MOTION_MEDIA_DIALOG_SURFACE_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";

type Props = {
  open: boolean;
  savedProgressSec: number | null;
  pending: boolean;
  onResume: () => void;
  onStartFromBeginning: () => void;
  restoreFocusRef?: RefObject<HTMLElement | null>;
};

export default function ResumeDialog({
  open,
  savedProgressSec,
  pending,
  onResume,
  onStartFromBeginning,
  restoreFocusRef,
}: Props) {
  const resumeButtonRef = useRef<HTMLButtonElement | null>(null);

  return (
    <Dialog open={open}>
      <DialogContent
        showCloseButton={false}
        className={cn(MOTION_MEDIA_DIALOG_SURFACE_CLASS)}
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
          <DialogTitle className="text-foreground">Resume movie?</DialogTitle>
          <DialogDescription className="text-muted-foreground">
            {savedProgressSec !== null
              ? `Resume from ${formatTimeSeconds(savedProgressSec)} or start from the beginning.`
              : "Resume your saved progress or start from the beginning."}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => {
              onStartFromBeginning();
            }}
            disabled={pending}
            className="border-border bg-muted text-muted-foreground hover:bg-accent hover:text-foreground"
          >
            {pending ? <Spinner className="size-4" aria-hidden="true" /> : null}
            {pending ? "Clearing progress..." : "Start from beginning"}
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
