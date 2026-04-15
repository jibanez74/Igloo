import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { deleteWatchRoom } from "@/lib/api";
import { WATCH_ROOM_KEY, WATCH_ROOMS_KEY } from "@/lib/constants";
import {
  showActionFailed,
  showSuccess,
} from "@/lib/toast-helpers";
import type { ApiResponseType, WatchRoomType } from "@/types";
import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Spinner } from "@/components/ui/spinner";

type DeleteWatchRoomDialogProps = {
  roomId: number;
  movieTitle: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDeleteStart?: () => void;
  onDeleteError?: () => void;
  onDeleted?: () => void | Promise<void>;
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
        return;
      }

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
    } catch {
      onDeleteError?.();
      showActionFailed(
        "close watch room",
        "An unexpected error occurred. Please try again.",
      );
    } finally {
      setDeleting(false);
      if (deleteSucceeded) {
        onOpenChange(false);
      }
    }
  }

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent className="border-slate-700 bg-slate-900">
        <AlertDialogHeader>
          <AlertDialogTitle className="text-white">
            Close watch room?
          </AlertDialogTitle>
          <AlertDialogDescription className="text-slate-400">
            This will close the watch room for{" "}
            <strong className="text-slate-200">{movieTitle}</strong> and remove
            all members. This action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel
            disabled={deleting}
            className="border-slate-700 bg-slate-800 text-slate-300 hover:bg-slate-700 hover:text-white"
          >
            Cancel
          </AlertDialogCancel>
          <Button
            type="button"
            variant="destructive"
            onClick={() => void handleDelete()}
            disabled={deleting}
            className="bg-red-600 text-white hover:bg-red-700"
          >
            {deleting ? <Spinner className="size-4" /> : null}
            {deleting ? "Closing…" : "Close room"}
          </Button>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
