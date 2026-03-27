import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { deleteMovie } from "@/lib/api";
import { LATEST_MOVIES_KEY, LIBRARY_MOVIE_DETAILS_KEY } from "@/lib/constants";
import { Spinner } from "@/components/ui/spinner";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";

type Props = {
  movieId: number;
  movieTitle: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

export default function DeleteMovieDialog({
  movieId,
  movieTitle,
  open,
  onOpenChange,
}: Props) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [deleteFile, setDeleteFile] = useState(false);
  const [deleting, setDeleting] = useState(false);

  async function handleDelete() {
    setDeleting(true);

    const res = await deleteMovie(movieId, deleteFile);
    setDeleting(false);

    if (res.error) {
      toast.error(res.message || "Failed to delete movie");
      return;
    }

    queryClient.invalidateQueries({ queryKey: [LATEST_MOVIES_KEY] });
    queryClient.removeQueries({
      queryKey: [LIBRARY_MOVIE_DETAILS_KEY, movieId],
    });

    toast.success(`"${movieTitle}" deleted successfully`);
    onOpenChange(false);
    navigate({ to: "/" });
  }

  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent className="border-slate-700 bg-slate-900">
        <AlertDialogHeader>
          <AlertDialogTitle className="text-white">
            Delete Movie
          </AlertDialogTitle>
          <AlertDialogDescription className="text-slate-400">
            Are you sure you want to delete{" "}
            <strong className="text-slate-200">{movieTitle}</strong>? This
            action cannot be undone.
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className="flex items-center gap-2 py-2">
          <Checkbox
            id="delete-file"
            checked={deleteFile}
            onCheckedChange={(checked) => setDeleteFile(checked === true)}
          />
          <Label htmlFor="delete-file" className="text-sm text-slate-300">
            Also delete the movie file from disk
          </Label>
        </div>

        <AlertDialogFooter>
          <AlertDialogCancel
            disabled={deleting}
            className="border-slate-700 bg-slate-800 text-slate-300 hover:bg-slate-700 hover:text-white"
          >
            Cancel
          </AlertDialogCancel>
          <AlertDialogAction
            onClick={handleDelete}
            disabled={deleting}
            className="bg-red-600 text-white hover:bg-red-700"
          >
            {deleting && <Spinner className="size-4" />}
            {deleting ? "Deleting…" : "Delete"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
