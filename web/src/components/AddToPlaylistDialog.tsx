import { useState, useMemo } from "react";
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
import { PLAYLISTS_KEY, PLAYLIST_TRACKS_KEY } from "@/lib/constants";
import LiveAnnouncer from "@/components/LiveAnnouncer";

type AddToPlaylistDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  trackId: number;
  trackTitle: string;
};

export default function AddToPlaylistDialog({
  open,
  onOpenChange,
  trackId,
  trackTitle,
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

  // Filter playlists that user can edit
  const editablePlaylists = useMemo(
    () => {
      const playlists = data?.error === false ? data.data.playlists : [];
      return playlists.filter((p) => p.can_edit);
    },
    [data]
  );

  // Filter by search query
  const filteredPlaylists = useMemo(() => {
    if (!searchQuery.trim()) return editablePlaylists;
    const query = searchQuery.toLowerCase();
    return editablePlaylists.filter((p) =>
      p.name.toLowerCase().includes(query)
    );
  }, [editablePlaylists, searchQuery]);

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
    setSearchQuery("");
    setSelectedPlaylists(new Set());
    setAnnouncement("");
    onOpenChange(false);
  };

  const togglePlaylist = (id: number, playlistName: string) => {
    setSelectedPlaylists((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
        setAnnouncement(`${playlistName} deselected. ${next.size} playlist${next.size !== 1 ? "s" : ""} selected.`);
      } else {
        next.add(id);
        setAnnouncement(`${playlistName} selected. ${next.size} playlist${next.size !== 1 ? "s" : ""} selected.`);
      }
      return next;
    });
  };

  const handleAdd = () => {
    if (selectedPlaylists.size === 0) return;
    mutation.mutate(Array.from(selectedPlaylists));
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="border-slate-700 bg-slate-900 sm:max-w-md">
        {/* Announce selection changes to screen readers */}
        <LiveAnnouncer message={announcement} />

        <DialogHeader>
          <DialogTitle className="text-white">Add to Playlist</DialogTitle>
          <DialogDescription className="text-slate-400">
            Add "{trackTitle}" to one or more playlists.
          </DialogDescription>
        </DialogHeader>

        {/* Search input */}
        {editablePlaylists.length > 5 && (
          <Input
            type="text"
            placeholder="Search playlists..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="border-slate-700 bg-slate-800 text-white placeholder:text-slate-500"
          />
        )}

        {/* Playlists list */}
        <div className="max-h-64 overflow-y-auto">
          {isLoading ? (
            <div className="flex justify-center py-8">
              <Spinner className="size-6 text-amber-400" />
            </div>
          ) : filteredPlaylists.length === 0 ? (
            <div className="rounded-lg border border-slate-700/50 bg-slate-800/50 py-8 text-center">
              <div className="mx-auto mb-3 flex size-12 items-center justify-center rounded-full bg-linear-to-br from-slate-700 via-slate-800 to-cyan-900/40">
                <ListMusic className="size-5 text-cyan-200/40" aria-hidden="true" />
              </div>
              <p className="text-slate-400">
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
                    className={`flex w-full items-center gap-3 rounded-lg px-3 py-2 text-left transition-colors ${
                      selectedPlaylists.has(playlist.id)
                        ? "bg-amber-500/20 text-white"
                        : "text-slate-300 hover:bg-slate-800"
                    }`}
                  >
                    <div
                      className={`flex size-5 items-center justify-center rounded-sm border ${
                        selectedPlaylists.has(playlist.id)
                          ? "border-amber-500 bg-amber-500"
                          : "border-slate-600"
                      }`}
                    >
                      {selectedPlaylists.has(playlist.id) && (
                        <Check className="size-3 text-slate-900" aria-hidden="true" />
                      )}
                    </div>
                    <div className="min-w-0 flex-1">
                      <p className="truncate font-medium">{playlist.name}</p>
                      <p className="text-xs text-slate-400">
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
            className="border-slate-600 bg-transparent text-slate-300 hover:bg-slate-800 hover:text-white"
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
