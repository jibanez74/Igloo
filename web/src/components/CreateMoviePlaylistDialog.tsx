import { useState, type RefObject } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  showActionFailed,
  showCreated,
  showValidationError,
} from "@/lib/toast-helpers";
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
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { createMoviePlaylist } from "@/lib/api";
import { MOVIE_PLAYLISTS_KEY } from "@/lib/constants";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";

type CreateMoviePlaylistDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  restoreFocusRef?: RefObject<HTMLElement | null>;
};

export default function CreateMoviePlaylistDialog({
  open,
  onOpenChange,
  restoreFocusRef,
}: CreateMoviePlaylistDialogProps) {
  const queryClient = useQueryClient();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");

  const mutation = useMutation({
    mutationFn: () =>
      createMoviePlaylist({
        name: name.trim(),
        description: description.trim() || undefined,
        is_public: false,
      }),
    onSuccess: (data) => {
      if (data.error) {
        showActionFailed("create playlist", data.message);
        return;
      }
      queryClient.invalidateQueries({ queryKey: [MOVIE_PLAYLISTS_KEY] });
      showCreated(
        "Playlist",
        `"${name.trim()}" has been created. Add movies from a movie's menu or this playlist page.`,
      );
      resetForm();
      onOpenChange(false);
    },
    onError: () => {
      showActionFailed(
        "create playlist",
        "An unexpected error occurred. Please try again.",
      );
    },
  });

  const resetForm = () => {
    setName("");
    setDescription("");
    mutation.reset();
  };

  const handleOpenChange = (next: boolean) => {
    if (!next) {
      resetForm();
    }
    onOpenChange(next);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      showValidationError("Playlist name is required");
      return;
    }
    if (name.trim().length > 255) {
      showValidationError("Playlist name is too long (max 255 characters)");
      return;
    }
    mutation.mutate();
  };

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        className="border-slate-700 bg-slate-900 sm:max-w-md"
        onCloseAutoFocus={
          restoreFocusRef
            ? event => {
                event.preventDefault();
                focusDialogRestoreTarget(restoreFocusRef.current);
              }
            : undefined
        }
      >
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle className="text-white">New movie playlist</DialogTitle>
            <DialogDescription className="text-slate-400">
              Create a playlist for movies. Track playlists stay on the Music page.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="movie-pl-name" className="text-slate-300">
                Name
              </Label>
              <Input
                id="movie-pl-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="border-slate-600 bg-slate-800 text-white"
                placeholder="My watchlist"
                autoComplete="off"
                maxLength={255}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="movie-pl-desc" className="text-slate-300">
                Description (optional)
              </Label>
              <Input
                id="movie-pl-desc"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                className="border-slate-600 bg-slate-800 text-white"
                placeholder="Optional notes"
                autoComplete="off"
                maxLength={1000}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              className="text-slate-400"
              onClick={() => handleOpenChange(false)}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={mutation.isPending}
              className="bg-amber-500 text-slate-900 hover:bg-amber-400"
            >
              {mutation.isPending ? (
                <>
                  <Spinner className="mr-2 size-4" />
                  Creating…
                </>
              ) : (
                "Create"
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
