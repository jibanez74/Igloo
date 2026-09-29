import { useState, type RefObject } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { deleteMovie } from "@/lib/api";
import { LATEST_MOVIES_KEY, LIBRARY_MOVIE_DETAILS_KEY } from "@/lib/constants";
import { showActionFailed, showDeleted } from "@/lib/toast-helpers";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import ConfirmDialog from "@/components/shared/ConfirmDialog";

type Props = {
  movieId: number;
  movieTitle: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  restoreFocusRef?: RefObject<HTMLElement | null>;
};

export default function DeleteMovieDialog({
  movieId,
  movieTitle,
  open,
  onOpenChange,
  restoreFocusRef,
}: Props) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [deleteFile, setDeleteFile] = useState(false);
  const [deleting, setDeleting] = useState(false);

  async function handleDelete() {
    setDeleting(true);

    try {
      const res = await deleteMovie(movieId, deleteFile);

      if (res.error) {
        showActionFailed("delete movie", res.message);
      } else {
        queryClient.invalidateQueries({ queryKey: [LATEST_MOVIES_KEY] });
        queryClient.removeQueries({
          queryKey: [LIBRARY_MOVIE_DETAILS_KEY, movieId],
        });

        showDeleted(`"${movieTitle}"`);
        onOpenChange(false);
        navigate({ to: "/" });
      }
    } catch {
      showActionFailed("delete movie");
    }

    setDeleting(false);
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      title="Delete Movie"
      description={
        <>
          Are you sure you want to delete{" "}
          <strong className="text-foreground">{movieTitle}</strong>? This action
          cannot be undone.
        </>
      }
      confirmLabel="Delete"
      pending={deleting}
      restoreFocusRef={restoreFocusRef}
      onConfirm={() => void handleDelete()}
    >
      <div className="flex items-center gap-2 py-2">
        <Checkbox
          id="delete-file"
          checked={deleteFile}
          onCheckedChange={(checked) => setDeleteFile(checked === true)}
        />
        <Label htmlFor="delete-file" className="text-sm text-muted-foreground">
          Also delete the movie file from disk
        </Label>
      </div>
    </ConfirmDialog>
  );
}
