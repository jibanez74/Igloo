import { useState, type RefObject } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useSessionMutation } from "@/hooks/useSessionMutation";
import { updateWatchRoomMembers } from "@/lib/api";
import { setWatchRoomMembers } from "@/lib/query-opts";
import { showActionFailed, showUpdated } from "@/lib/toast-helpers";
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
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import WatchRoomInviteePicker from "@/components/watch-room/WatchRoomInviteePicker";

type EditWatchRoomMembersDialogProps = {
  roomId: number;
  movieTitle: string;
  /** The members besides the owner, as the room has them now. */
  inviteeIds: number[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
  restoreFocusRef?: RefObject<HTMLElement | null>;
};

function sameIds(a: number[], b: number[]) {
  return a.length === b.length && a.every(id => b.includes(id));
}

/**
 * The owner changes who is invited. Anyone removed loses the room at once,
 * on every device; anyone added finds it in their Watch Rooms.
 */
export default function EditWatchRoomMembersDialog({
  roomId,
  movieTitle,
  inviteeIds,
  open,
  onOpenChange,
  restoreFocusRef,
}: EditWatchRoomMembersDialogProps) {
  const queryClient = useQueryClient();
  const [selectedUserIds, setSelectedUserIds] = useState<number[]>(inviteeIds);
  // Each opening starts from the room's current invitees.
  const [prevOpen, setPrevOpen] = useState(open);
  if (open !== prevOpen) {
    setPrevOpen(open);
    if (open) setSelectedUserIds(inviteeIds);
  }

  const mutation = useSessionMutation({
    mutationFn: (ids: number[]) =>
      updateWatchRoomMembers(roomId, { invited_user_ids: ids }),
    onSuccess: data => {
      if (data.error) {
        showActionFailed("update the invitees", data);
        return;
      }
      setWatchRoomMembers(queryClient, roomId, data.data.room.members);
      showUpdated("Invitees", `The room for "${movieTitle}" is up to date.`);
      onOpenChange(false);
    },
    onError: () => {
      showActionFailed("update the invitees");
    },
  });

  const unchanged = sameIds(selectedUserIds, inviteeIds);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        size="xl"
        className="flex max-h-[90vh] flex-col overflow-hidden"
        onCloseAutoFocus={event => {
          if (!restoreFocusRef) return;
          event.preventDefault();
          focusDialogRestoreTarget(restoreFocusRef.current);
        }}
      >
        <form
          className="flex min-h-0 flex-1 flex-col"
          onSubmit={event => {
            event.preventDefault();
            if (unchanged) return;
            mutation.mutate(selectedUserIds);
          }}
        >
          <DialogHeader>
            <DialogTitle>Edit invitees</DialogTitle>
            <DialogDescription>
              Choose who can join the room for <strong>{movieTitle}</strong>.
              Anyone you remove leaves the room right away.
            </DialogDescription>
          </DialogHeader>

          <div className="min-h-0 flex-1 overflow-y-auto py-4 pr-1">
            <WatchRoomInviteePicker
              enabled={open}
              selectedUserIds={selectedUserIds}
              onSelectedUserIdsChange={setSelectedUserIds}
              description="Everyone checked here can join and control playback."
            />
          </div>

          <DialogFooter className="border-t border-border pt-4">
            <Button
              type="button"
              variant="ghost-muted"
              onClick={() => onOpenChange(false)}
              disabled={mutation.isPending}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              variant="accent"
              disabled={mutation.isPending || unchanged}
            >
              {mutation.isPending ? (
                <>
                  <Spinner />
                  Saving…
                </>
              ) : (
                "Save invitees"
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
