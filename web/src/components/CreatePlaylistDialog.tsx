import type { RefObject } from "react";
import PlaylistFormDialog from "./PlaylistFormDialog";

type CreatePlaylistDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  restoreFocusRef?: RefObject<HTMLElement | null>;
};

export default function CreatePlaylistDialog({
  open,
  onOpenChange,
  restoreFocusRef,
}: CreatePlaylistDialogProps) {
  return (
    <PlaylistFormDialog
      mode="create"
      open={open}
      onOpenChange={onOpenChange}
      restoreFocusRef={restoreFocusRef}
    />
  );
}
