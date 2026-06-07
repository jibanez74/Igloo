import type { RefObject } from "react";
import { Link } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import TmdbMoviePicker from "@/components/TmdbMoviePicker";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import { createNotification } from "@/lib/api";
import { authUserQueryOpts } from "@/lib/query-opts";
import {
  showActionFailed,
  showCreated,
} from "@/lib/toast-helpers";
import type { TmdbSearchResultType } from "@/types";

const MOVIE_REQUEST_NOTIFICATION_TITLE = "movie_request";

type RequestMovieDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  restoreFocusRef?: RefObject<HTMLElement | null>;
};

export default function RequestMovieDialog({
  open,
  onOpenChange,
  restoreFocusRef,
}: RequestMovieDialogProps) {
  const { data: authData } = useQuery(authUserQueryOpts());

  async function handleConfirm(selectedResult: TmdbSearchResultType) {
    if (authData?.error !== false) {
      showActionFailed(
        "send movie request",
        "Your account details are unavailable right now.",
      );
      return;
    }

    const requester = authData.data.user;
    const releaseYear = selectedResult.release_date?.slice(0, 4).trim();
    const lines = [
      `Requester: ${requester.name} <${requester.email}>`,
      `Movie: ${selectedResult.title}`,
      releaseYear ? `Year: ${releaseYear}` : null,
      `TMDB ID: ${selectedResult.tmdb_id}`,
      `TMDB URL: https://www.themoviedb.org/movie/${selectedResult.tmdb_id}`,
    ].filter(Boolean);

    const response = await createNotification({
      title: MOVIE_REQUEST_NOTIFICATION_TITLE,
      message: lines.join("\n"),
      isAdmin: true,
    });

    if (response.error) {
      showActionFailed("send movie request", response.message);
      return;
    }

    onOpenChange(false);
    window.setTimeout(() => {
      showCreated(
        "Movie request",
        `"${selectedResult.title}" was sent to the admin notification queue.`,
      );
    }, 0);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="max-h-[85vh] overflow-y-auto border-slate-700 bg-slate-900 sm:max-w-2xl"
        onCloseAutoFocus={
          restoreFocusRef
            ? event => {
                event.preventDefault();
                focusDialogRestoreTarget(restoreFocusRef.current);
              }
            : undefined
        }
      >
        <DialogHeader>
          <DialogTitle className="text-white">Request Movie</DialogTitle>
          <DialogDescription className="text-slate-400">
            Search TMDB, pick the exact movie you want, and send the request to an admin.
          </DialogDescription>
        </DialogHeader>

        <TmdbMoviePicker
          confirmLabel="Send Request"
          initialTitle=""
          initialYear=""
          isResultBlocked={result => result.already_in_library}
          onConfirm={handleConfirm}
          renderResultMeta={result => {
            if (!result.already_in_library || result.library_movie_id == null) {
              return null;
            }

            return (
              <p className="text-amber-300">
                This movie is already in your library.
                {" "}
                <Link
                  to="/movies/$id"
                  params={{ id: String(result.library_movie_id) }}
                  className="underline decoration-amber-300/60 underline-offset-2 hover:text-amber-200"
                >
                  Open existing movie
                </Link>
              </p>
            );
          }}
        />
      </DialogContent>
    </Dialog>
  );
}
