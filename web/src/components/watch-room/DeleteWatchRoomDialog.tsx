import { useState, type RefObject } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { deleteWatchRoom } from "@/lib/api";
import { forgetWatchRoom } from "@/lib/query-opts";
import {
  showActionFailed,
  showSuccess,
} from "@/lib/toast-helpers";
import ConfirmDialog from "@/components/shared/ConfirmDialog";

type DeleteWatchRoomDialogProps = {
  roomId: number;
  movieTitle: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDeleteStart?: () => void;
  onDeleteError?: () => void;
  onDeleted?: () => void | Promise<void>;
  restoreFocusRef?: RefObject<HTMLElement | null>;
};

export default function DeleteWatchRoomDialog({
  roomId,
  movieTitle,
  open,
  onOpenChange,
  onDeleteStart,
  onDeleteError,
  onDeleted,
  restoreFocusRef,
}: DeleteWatchRoomDialogProps) {
  const queryClient = useQueryClient();
  const [deleting, setDeleting] = useState(false);

  async function handleDelete() {
    setDeleting(true);
    onDeleteStart?.();
    let deleteSucceeded = false;

    try {
      const res = await deleteWatchRoom(roomId);
      if (res.error) {
        onDeleteError?.();
        showActionFailed("close watch room", res);
      } else {
        showSuccess(
          "Watch room closed",
          `"${movieTitle}" is no longer available.`,
        );
        deleteSucceeded = true;

        // onDeleted leaves the room's page when the dialog sits on it, which
        // has to happen before the room's own cache entry goes.
        try {
          await forgetWatchRoom(queryClient, roomId, onDeleted);
        } catch (error) {
          console.error("DeleteWatchRoomDialog onDeleted failed", error);
          showActionFailed(
            "finish closing watch room",
            "The room was closed, but the follow-up action failed.",
          );
        }
      }
    } catch {
      onDeleteError?.();
      showActionFailed(
        "close watch room",
        "An unexpected error occurred. Please try again.",
      );
    }

    setDeleting(false);
    if (deleteSucceeded) {
      onOpenChange(false);
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Close watch room?"
      description={
        <>
          This will close the watch room for{" "}
          <strong className="text-foreground">{movieTitle}</strong> and remove
          all members. This action cannot be undone.
        </>
      }
      confirmLabel="Close room"
      pending={deleting}
      restoreFocusRef={restoreFocusRef}
      onConfirm={() => void handleDelete()}
    />
  );
}
