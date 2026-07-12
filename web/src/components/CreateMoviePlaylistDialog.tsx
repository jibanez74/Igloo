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
import {
  MOVIE_PLAYLISTS_KEY,
  PLAYLIST_DESCRIPTION_MAX_LENGTH,
  PLAYLIST_NAME_MAX_LENGTH,
} from "@/lib/constants";
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
    if (name.trim().length > PLAYLIST_NAME_MAX_LENGTH) {
      showValidationError(
        `Playlist name is too long (max ${PLAYLIST_NAME_MAX_LENGTH} characters)`,
      );
      return;
    }
    mutation.mutate();
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
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle className="text-foreground">New movie playlist</DialogTitle>
            <DialogDescription className="text-muted-foreground">
              Create a playlist for movies. Track playlists stay on the Music page.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid gap-2">
              <Label htmlFor="movie-pl-name" className="text-muted-foreground">
                Name
              </Label>
              <Input
                id="movie-pl-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="border-border bg-muted text-foreground"
                placeholder="My watchlist"
                autoComplete="off"
                maxLength={PLAYLIST_NAME_MAX_LENGTH}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="movie-pl-desc" className="text-muted-foreground">
                Description (optional)
              </Label>
              <Input
                id="movie-pl-desc"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                className="border-border bg-muted text-foreground"
                placeholder="Optional notes"
                autoComplete="off"
                maxLength={PLAYLIST_DESCRIPTION_MAX_LENGTH}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              className="text-muted-foreground"
              onClick={() => handleOpenChange(false)}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              disabled={mutation.isPending}
              className="bg-primary text-primary-foreground hover:bg-primary/90"
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
