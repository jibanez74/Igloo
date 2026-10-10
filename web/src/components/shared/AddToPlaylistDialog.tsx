import { useState, type RefObject } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { showAdded, showActionFailed, showInfo } from "@/lib/toast-helpers";
import { Check, ListMusic, ListVideo, type LucideIcon } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { playlistPickerQueryOpts } from "@/lib/query-opts";
import { PLAYLIST_KINDS, type PlaylistKind } from "@/lib/playlist-kinds";
import { capitalize, pluralize } from "@/lib/format";
import { MOTION_MICRO_COLORS_CLASS } from "@/lib/constants";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import type { ApiFailureType } from "@/types";

const PICKER_EMPTY_ICONS: Record<PlaylistKind, LucideIcon> = {
  music: ListMusic,
  movie: ListVideo,
};

type AddToPlaylistDialogProps = {
  kind: PlaylistKind;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The track or movie being added. */
  itemId: number;
  itemTitle: string;
  restoreFocusRef?: RefObject<HTMLElement | null>;
};

/**
 * The picker a track or a movie joins playlists through: the viewer's
 * editable playlists of that kind, multi-selected, then one request per
 * playlist. The server reports duplicates as skipped rather than failing, so
 * "already in" is only known afterwards.
 */
export default function AddToPlaylistDialog({
  kind,
  open,
  onOpenChange,
  itemId,
  itemTitle,
  restoreFocusRef,
}: AddToPlaylistDialogProps) {
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedPlaylists, setSelectedPlaylists] = useState<Set<number>>(
    new Set()
  );
  const [announcement, setAnnouncement] = useState("");
  const queryClient = useQueryClient();
  const { addItems, listKey, detailsKey, itemsKey, itemNoun } =
    PLAYLIST_KINDS[kind];
  const itemLabel = capitalize(itemNoun.singular);
  const EmptyIcon = PICKER_EMPTY_ICONS[kind];

  const { data: playlists = [], isLoading } = useQuery({
    ...playlistPickerQueryOpts(kind),
    enabled: open,
  });

  const editablePlaylists = playlists.filter((p) => p.can_edit);

  const filteredPlaylists = !searchQuery.trim()
    ? editablePlaylists
    : editablePlaylists.filter((p) =>
        p.name.toLowerCase().includes(searchQuery.toLowerCase())
      );

  const mutation = useMutation({
    mutationFn: (playlistIds: number[]) =>
      Promise.all(playlistIds.map((id) => addItems(id, [itemId]))),
    onSuccess: (results, playlistIds) => {
      // The client answers with an envelope rather than throwing, so each
      // playlist's outcome is judged here. Refresh whatever grew before
      // reporting: a batch can add to one playlist and be refused by another.
      let totalAdded = 0;
      results.forEach((result, index) => {
        if (result.error || result.data.added === 0) return;
        totalAdded += result.data.added;
        queryClient.invalidateQueries({
          queryKey: [itemsKey, playlistIds[index]],
        });
        queryClient.invalidateQueries({
          queryKey: [detailsKey, playlistIds[index]],
        });
      });
      if (totalAdded > 0) {
        queryClient.invalidateQueries({ queryKey: [listKey] });
      }

      const failure = results.find(
        (result): result is ApiFailureType => result.error,
      );
      if (failure) {
        showActionFailed(`add ${itemNoun.singular} to playlists`, failure);
        return;
      }

      if (totalAdded > 0) {
        showAdded(itemLabel, `to ${pluralize(playlistIds.length, "playlist")}`);
        // Not handleClose: this render still reports the mutation pending.
        resetAndClose();
      } else {
        showInfo(`${itemLabel} already in selected playlists`);
      }
    },
    onError: () => {
      showActionFailed(`add ${itemNoun.singular} to playlists`);
    },
  });

  const resetAndClose = () => {
    setSearchQuery("");
    setSelectedPlaylists(new Set());
    setAnnouncement("");
    onOpenChange(false);
  };

  // Cancel and the overlay's own dismissals hold while a request is in flight.
  const handleClose = () => {
    if (mutation.isPending) {
      return;
    }
    resetAndClose();
  };

  const handleOpenChange = (next: boolean) => {
    if (next) {
      onOpenChange(true);
      return;
    }
    handleClose();
  };

  const togglePlaylist = (id: number, playlistName: string) => {
    // Computed outside the updater: React may invoke an updater more than once,
    // and announcing from inside it is a render-phase update.
    const next = new Set(selectedPlaylists);
    const wasSelected = next.delete(id);
    if (!wasSelected) {
      next.add(id);
    }

    setSelectedPlaylists(next);
    setAnnouncement(
      wasSelected
        ? `${playlistName} deselected. ${pluralize(next.size, "playlist")} selected.`
        : `${playlistName} selected. ${pluralize(next.size, "playlist")} selected.`,
    );
  };

  const handleAdd = () => {
    if (selectedPlaylists.size === 0) return;
    mutation.mutate(Array.from(selectedPlaylists));
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        className="border-border bg-card sm:max-w-md"
        onCloseAutoFocus={
          restoreFocusRef
            ? event => {
                event.preventDefault();
                focusDialogRestoreTarget(restoreFocusRef.current);
              }
            : undefined
        }
      >
        {/* Announce selection changes to screen readers */}
        <LiveAnnouncer message={announcement} />

        <DialogHeader>
          <DialogTitle className="text-foreground">Add to Playlist</DialogTitle>
          <DialogDescription className="text-muted-foreground">
            Add "{itemTitle}" to one or more playlists.
          </DialogDescription>
        </DialogHeader>

        {/* Search input */}
        {editablePlaylists.length > 5 && (
          <Input
            type="text"
            placeholder="Search playlists..."
            aria-label="Search playlists"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="border-border bg-muted text-foreground placeholder:text-muted-foreground"
          />
        )}

        {/* Playlists list */}
        <div className="max-h-64 overflow-y-auto">
          {isLoading ? (
            <div className="flex justify-center py-8">
              <Spinner className="size-6 text-primary" />
            </div>
          ) : filteredPlaylists.length === 0 ? (
            <div className="rounded-lg border border-border/50 bg-muted/50 py-8 text-center">
              <div className="mx-auto mb-3 flex size-12 items-center justify-center rounded-full bg-linear-to-br from-muted via-muted to-primary/40">
                <EmptyIcon className="size-5 text-primary/40" aria-hidden="true" />
              </div>
              <p className="text-muted-foreground">
                {editablePlaylists.length === 0
                  ? "No playlists yet. Create one to get started."
                  : "No playlists match your search."}
              </p>
            </div>
          ) : (
            <ul className="space-y-1">
              {filteredPlaylists.map((playlist) => (
                <li key={playlist.id}>
                  <button
                    type="button"
                    onClick={() => togglePlaylist(playlist.id, playlist.name)}
                    aria-pressed={selectedPlaylists.has(playlist.id)}
                    className={`flex w-full items-center gap-3 rounded-lg px-3 py-2 text-left ${MOTION_MICRO_COLORS_CLASS} ${
                      selectedPlaylists.has(playlist.id)
                        ? "bg-primary/20 text-foreground"
                        : "text-muted-foreground hover:bg-muted"
                    }`}
                  >
                    <div
                      className={`flex size-5 items-center justify-center rounded-sm border ${
                        selectedPlaylists.has(playlist.id)
                          ? "border-primary bg-primary"
                          : "border-border"
                      }`}
                    >
                      {selectedPlaylists.has(playlist.id) && (
                        <Check className="size-3 text-primary-foreground" aria-hidden="true" />
                      )}
                    </div>
                    <div className="min-w-0 flex-1">
                      <p className="truncate font-medium">{playlist.name}</p>
                      <p className="text-xs text-muted-foreground">
                        {pluralize(playlist.item_count, itemNoun.singular)}
                      </p>
                    </div>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>

        <DialogFooter className="gap-2 sm:gap-0">
          <Button
            type="button"
            variant="outline"
            onClick={handleClose}
            disabled={mutation.isPending}
            className="border-border bg-transparent text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            Cancel
          </Button>
          <Button
            type="button"
            variant="accent"
            onClick={handleAdd}
            disabled={mutation.isPending || selectedPlaylists.size === 0}
          >
            {mutation.isPending ? (
              <>
                <Spinner className="mr-2 size-4" />
                Adding...
              </>
            ) : (
              selectedPlaylists.size === 0
                ? "Add to Playlists"
                : `Add to ${pluralize(selectedPlaylists.size, "Playlist")}`
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
