import { useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useSessionMutation } from "@/hooks/useSessionMutation";
import { useNavigate } from "@tanstack/react-router";
import { Pencil, Trash2 } from "lucide-react";
import ConfirmDialog from "@/components/shared/ConfirmDialog";
import PlaylistFormDialog from "@/components/shared/PlaylistFormDialog";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { PLAYLIST_KINDS, type PlaylistKind } from "@/lib/playlist-kinds";
import { showActionFailed, showDeleted } from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import type { ApiResponseType, NullableString } from "@/types";

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

// Either library's playlists list, as far as removing one row needs to know.
type PlaylistsListData = ApiResponseType<{ playlists: { id: number }[] }>;

const withoutPlaylist = (list: PlaylistsListData | undefined, id: number) =>
  list?.error === false
    ? {
        ...list,
        data: {
          ...list.data,
          playlists: list.data.playlists.filter(row => row.id !== id),
        },
      }
    : list;

// The owner-only Edit and Delete row of a playlist page, with the dialogs
// behind them. They are ghost buttons, so a caller that aligns the row with
// the text above pulls it back by the buttons' padding (-ml-2.5). Edit reuses the create dialog; Delete confirms, then returns
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

  const deleteMutation = useSessionMutation({
    mutationFn: () => api.remove(playlist.id),
    onSuccess: async (result) => {
      if (result.error) {
        showActionFailed("delete playlist", result);
        return;
      }
      showDeleted("Playlist");
      // The Playlists tab renders from the cached list on arrival, so the
      // deleted card goes before the page does.
      queryClient.setQueryData<PlaylistsListData>([api.listKey], list =>
        withoutPlaylist(list, playlist.id),
      );
      void queryClient.invalidateQueries({ queryKey: [api.listKey] });
      // The playlist's own queries are observed until its page goes; removing
      // them before would refetch the deleted playlist.
      await navigate(api.index);
      queryClient.removeQueries({ queryKey: [api.detailsKey, playlist.id] });
      queryClient.removeQueries({ queryKey: [api.itemsKey, playlist.id] });
    },
    onError: () => {
      showActionFailed("delete playlist");
    },
  });

  return (
    <>
      <div className={cn("flex flex-wrap gap-2", className)}>
        <Button
          ref={editButtonRef}
          variant="ghost-muted"
          size="sm"
          onClick={() => setShowEditDialog(true)}
          aria-label="Edit playlist"
        >
          <Pencil className="size-4" aria-hidden="true" />
          <span>
            Edit<span className="hidden sm:inline"> Details</span>
          </span>
        </Button>
        <Button
          ref={deleteButtonRef}
          variant="ghost-destructive"
          size="sm"
          onClick={() => setShowDeleteDialog(true)}
          disabled={deleteMutation.isPending}
          aria-label="Delete playlist"
        >
          {deleteMutation.isPending ? (
            <Spinner />
          ) : (
            <Trash2 className="size-4" aria-hidden="true" />
          )}
          <span>
            Delete<span className="hidden sm:inline"> Playlist</span>
          </span>
        </Button>
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
