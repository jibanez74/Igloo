import { useState, type RefObject } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { deleteWatchRoom } from "@/lib/api";
import { WATCH_ROOM_KEY, WATCH_ROOMS_KEY } from "@/lib/constants";
import {
  showActionFailed,
  showSuccess,
} from "@/lib/toast-helpers";
import type { ApiResponseType, WatchRoomType } from "@/types";
import ConfirmDialog from "@/components/ConfirmDialog";

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

function removeDeletedRoomFromCache(
  existing:
    | ApiResponseType<{ rooms: WatchRoomType[] }>
    | undefined,
  roomId: number,
) {
  if (!existing || existing.error) {
    return existing;
  }

  return {
    ...existing,
    data: {
      ...existing.data,
      rooms: existing.data.rooms.filter(room => room.id !== roomId),
    },
  };
}

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
        showActionFailed("close watch room", res.message);
      } else {
        queryClient.setQueryData(
          [WATCH_ROOMS_KEY],
          (
            existing:
              | ApiResponseType<{ rooms: WatchRoomType[] }>
              | undefined,
          ) => removeDeletedRoomFromCache(existing, roomId),
        );
        queryClient.removeQueries({
          queryKey: [WATCH_ROOM_KEY, roomId],
          exact: true,
        });
        void queryClient.invalidateQueries({ queryKey: [WATCH_ROOMS_KEY] });

        showSuccess(
          "Watch room closed",
          `"${movieTitle}" is no longer available.`,
        );
        deleteSucceeded = true;

        if (onDeleted) {
          try {
            await onDeleted();
          } catch (error) {
            console.error("DeleteWatchRoomDialog onDeleted failed", error);
            showActionFailed(
              "finish closing watch room",
              "The room was closed, but the follow-up action failed.",
            );
          }
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
