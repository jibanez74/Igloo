import { useState, type RefObject } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { showAdded, showActionFailed, showInfo } from "@/lib/toast-helpers";
import { ListMusic, Check } from "lucide-react";
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
import { playlistsQueryOpts } from "@/lib/query-opts";
import { addTracksToPlaylist } from "@/lib/api";
import {
  MOTION_MICRO_COLORS_CLASS,
  PLAYLISTS_KEY,
  PLAYLIST_TRACKS_KEY,
} from "@/lib/constants";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";

type AddToPlaylistDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  trackId: number;
  trackTitle: string;
  restoreFocusRef?: RefObject<HTMLElement | null>;
};

export default function AddToPlaylistDialog({
  open,
  onOpenChange,
  trackId,
  trackTitle,
  restoreFocusRef,
}: AddToPlaylistDialogProps) {
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedPlaylists, setSelectedPlaylists] = useState<Set<number>>(
    new Set()
  );
  const [announcement, setAnnouncement] = useState("");
  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery({
    ...playlistsQueryOpts(),
    enabled: open,
  });

  const playlists = data?.error === false ? data.data.playlists : [];
  const editablePlaylists = playlists.filter((p) => p.can_edit);

  const filteredPlaylists = !searchQuery.trim()
    ? editablePlaylists
    : editablePlaylists.filter((p) =>
        p.name.toLowerCase().includes(searchQuery.toLowerCase())
      );

  const mutation = useMutation({
    mutationFn: async (playlistIds: number[]) => {
      const results = await Promise.all(
        playlistIds.map((id) => addTracksToPlaylist(id, [trackId]))
      );
      return results;
    },
    onSuccess: (results) => {
      const totalAdded = results.reduce(
        (sum, r) => (r.error === false ? sum + (r.data?.added ?? 0) : sum),
        0
      );

      if (totalAdded > 0) {
        // Invalidate affected playlist queries
        selectedPlaylists.forEach((id) => {
          queryClient.invalidateQueries({
            queryKey: [PLAYLIST_TRACKS_KEY, id],
          });
        });
        queryClient.invalidateQueries({ queryKey: [PLAYLISTS_KEY] });

        showAdded("Track", `to ${selectedPlaylists.size} playlist(s)`);
        handleClose();
      } else {
        showInfo("Track already in selected playlists");
      }
    },
    onError: () => {
      showActionFailed("add track to playlists");
    },
  });

  const handleClose = () => {
    if (mutation.isPending) {
      return;
    }

    setSearchQuery("");
    setSelectedPlaylists(new Set());
    setAnnouncement("");
    onOpenChange(false);
  };

  const handleOpenChange = (next: boolean) => {
    if (!next && mutation.isPending) {
      return;
    }

    if (!next) {
      setSearchQuery("");
      setSelectedPlaylists(new Set());
      setAnnouncement("");
    }
    onOpenChange(next);
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
        ? `${playlistName} deselected. ${next.size} playlist${next.size !== 1 ? "s" : ""} selected.`
        : `${playlistName} selected. ${next.size} playlist${next.size !== 1 ? "s" : ""} selected.`,
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
            Add "{trackTitle}" to one or more playlists.
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
                <ListMusic className="size-5 text-primary/40" aria-hidden="true" />
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
                        {playlist.track_count} tracks
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
              `Add to ${selectedPlaylists.size || ""} Playlist${selectedPlaylists.size !== 1 ? "s" : ""}`
            )}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
