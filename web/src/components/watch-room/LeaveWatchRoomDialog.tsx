import { useState, type RefObject } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { leaveWatchRoom } from "@/lib/api";
import { forgetWatchRoom } from "@/lib/query-opts";
import { showActionFailed, showSuccess } from "@/lib/toast-helpers";
import ConfirmDialog from "@/components/shared/ConfirmDialog";
import { sessionEndedSince, sessionStamp } from "@/lib/session";

type LeaveWatchRoomDialogProps = {
  roomId: number;
  movieTitle: string;
  hostName: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onLeaveStart?: () => void;
  onLeaveError?: () => void;
  /** Runs before the room's cache entry goes, so its page can leave first. */
  onLeft?: () => void | Promise<void>;
  restoreFocusRef?: RefObject<HTMLElement | null>;
};

/** An invitee leaves a room for good: it drops off their list on every device. */
export default function LeaveWatchRoomDialog({
  roomId,
  movieTitle,
  hostName,
  open,
  onOpenChange,
  onLeaveStart,
  onLeaveError,
  onLeft,
  restoreFocusRef,
}: LeaveWatchRoomDialogProps) {
  const queryClient = useQueryClient();
  const [leaving, setLeaving] = useState(false);

  async function handleLeave() {
    setLeaving(true);
    onLeaveStart?.();
    let left = false;
    const session = sessionStamp();

    try {
      const res = await leaveWatchRoom(roomId);
      // Signed out meanwhile: the next reader's cache and page are not ours.
      if (sessionEndedSince(session)) return;
      if (res.error) {
        onLeaveError?.();
        showActionFailed("leave the watch room", res);
      } else {
        showSuccess("Left watch room", `"${movieTitle}" is off your list.`);
        left = true;
        try {
          await forgetWatchRoom(queryClient, roomId, onLeft);
        } catch (error) {
          console.error("LeaveWatchRoomDialog onLeft failed", error);
        }
      }
    } catch {
      onLeaveError?.();
      showActionFailed("leave the watch room");
    }

    setLeaving(false);
    if (left) {
      onOpenChange(false);
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Leave watch room?"
      description={
        <>
          You will leave the room for{" "}
          <strong className="text-foreground">{movieTitle}</strong> and it will
          drop off your list. Only {hostName} can invite you back.
        </>
      }
      confirmLabel="Leave room"
      pending={leaving}
      restoreFocusRef={restoreFocusRef}
      onConfirm={() => void handleLeave()}
    />
  );
}
