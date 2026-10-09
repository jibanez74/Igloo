import { useRef, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Pencil, Trash2 } from "lucide-react";
import ConfirmDialog from "@/components/shared/ConfirmDialog";
import PlaylistFormDialog from "@/components/shared/PlaylistFormDialog";
import { Spinner } from "@/components/ui/spinner";
import {
  FOCUS_VISIBLE_RING_CLASS,
  MOTION_MICRO_COLORS_CLASS,
} from "@/lib/constants";
import { PLAYLIST_KINDS, type PlaylistKind } from "@/lib/playlist-kinds";
import { showActionFailed, showDeleted } from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import type { NullableString } from "@/types";

type OwnedPlaylist = {
  id: number;
  name: string;
  description: NullableString;
  cover_image: NullableString;
  is_public: boolean;
};

type PlaylistOwnerActionsProps = {
  kind: PlaylistKind;
  playlist: OwnedPlaylist;
  className?: string;
};

const ACTION_BUTTON_CLASS =
  "inline-flex items-center gap-1.5 rounded-sm text-xs text-muted-foreground sm:gap-2 sm:text-sm";

// The owner-only Edit and Delete row of a playlist page, with the dialogs
// behind them. Edit reuses the create dialog; Delete confirms, then returns
// to that library's Playlists tab. Callers render it only for the owner.
export default function PlaylistOwnerActions({
  kind,
  playlist,
  className,
}: PlaylistOwnerActionsProps) {
  const api = PLAYLIST_KINDS[kind];
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [showEditDialog, setShowEditDialog] = useState(false);
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const editButtonRef = useRef<HTMLButtonElement | null>(null);
  const deleteButtonRef = useRef<HTMLButtonElement | null>(null);

  const deleteMutation = useMutation({
    mutationFn: () => api.remove(playlist.id),
    onSuccess: (result) => {
      if (result.error) {
        showActionFailed("delete playlist", result.message);
        return;
      }
      queryClient.invalidateQueries({ queryKey: [api.listKey] });
      showDeleted("Playlist");
      navigate(api.index);
    },
    onError: () => {
      showActionFailed("delete playlist");
    },
  });

  return (
    <>
      <div className={cn("flex flex-wrap gap-3 sm:gap-4", className)}>
        <button
          type="button"
          ref={editButtonRef}
          onClick={() => setShowEditDialog(true)}
          className={cn(
            MOTION_MICRO_COLORS_CLASS,
            FOCUS_VISIBLE_RING_CLASS,
            ACTION_BUTTON_CLASS,
            "hover:text-primary focus-visible:text-primary",
          )}
          aria-label="Edit playlist"
        >
          <Pencil className="size-4" aria-hidden="true" />
          <span>
            Edit<span className="hidden sm:inline"> Details</span>
          </span>
        </button>
        <button
          type="button"
          ref={deleteButtonRef}
          onClick={() => setShowDeleteDialog(true)}
          disabled={deleteMutation.isPending}
          className={cn(
            MOTION_MICRO_COLORS_CLASS,
            FOCUS_VISIBLE_RING_CLASS,
            ACTION_BUTTON_CLASS,
            "hover:text-destructive focus-visible:text-destructive disabled:opacity-50",
          )}
          aria-label="Delete playlist"
        >
          {deleteMutation.isPending ? (
            <Spinner className="size-4" />
          ) : (
            <Trash2 className="size-4" aria-hidden="true" />
          )}
          <span>
            Delete<span className="hidden sm:inline"> Playlist</span>
          </span>
        </button>
      </div>

      <PlaylistFormDialog
        kind={kind}
        mode="edit"
        open={showEditDialog}
        onOpenChange={setShowEditDialog}
        playlist={playlist}
        restoreFocusRef={editButtonRef}
      />

      <ConfirmDialog
        open={showDeleteDialog}
        onOpenChange={setShowDeleteDialog}
        title="Delete playlist"
        description={
          <>
            Are you sure you want to delete &ldquo;{playlist.name}&rdquo;? This
            action cannot be undone.
          </>
        }
        confirmLabel="Delete"
        pending={deleteMutation.isPending}
        restoreFocusRef={deleteButtonRef}
        onConfirm={() => deleteMutation.mutate()}
      />
    </>
  );
}
