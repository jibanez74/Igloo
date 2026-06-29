import type { RefObject } from "react";
import { useNavigate } from "@tanstack/react-router";
import { useQuery } from "@tanstack/react-query";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import SpotifyAlbumPicker from "@/components/SpotifyAlbumPicker";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import { createNotification } from "@/lib/api";
import { authUserQueryOpts } from "@/lib/query-opts";
import {
  showActionFailed,
  showCreated,
} from "@/lib/toast-helpers";
import type { SpotifyAlbumSearchResultType } from "@/types";

const ALBUM_REQUEST_NOTIFICATION_TITLE = "album_request";

type RequestAlbumDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  restoreFocusRef?: RefObject<HTMLElement | null>;
};

export default function RequestAlbumDialog({
  open,
  onOpenChange,
  restoreFocusRef,
}: RequestAlbumDialogProps) {
  const navigate = useNavigate();
  const { data: authData } = useQuery(authUserQueryOpts());

  async function handleConfirm(selectedResult: SpotifyAlbumSearchResultType) {
    if (selectedResult.already_in_library) {
      if (selectedResult.library_album_id == null) {
        showActionFailed(
          "open existing album",
          "The matching album exists, but its page could not be identified.",
        );
        return;
      }

      onOpenChange(false);
      void navigate({
        to: "/music/album/$id",
        params: { id: String(selectedResult.library_album_id) },
      });
      return;
    }

    if (authData?.error !== false) {
      showActionFailed(
        "send album request",
        "Your account details are unavailable right now.",
      );
      return;
    }

    const requester = authData.data.user;
    const spotifyURL =
      selectedResult.spotify_url ||
      `https://open.spotify.com/album/${selectedResult.spotify_id}`;
    const artists = selectedResult.artist_names.join(", ");
    const lines = [
      `Requester: ${requester.name} <${requester.email}>`,
      `Album: ${selectedResult.title}`,
      artists ? `Artists: ${artists}` : null,
      selectedResult.release_date
        ? `Release date: ${selectedResult.release_date}`
        : null,
      selectedResult.total_tracks > 0
        ? `Total tracks: ${selectedResult.total_tracks}`
        : null,
      `Spotify ID: ${selectedResult.spotify_id}`,
      `Spotify URL: ${spotifyURL}`,
    ].filter(Boolean);

    const response = await createNotification({
      title: ALBUM_REQUEST_NOTIFICATION_TITLE,
      message: lines.join("\n"),
      isAdmin: true,
    });

    if (response.error) {
      showActionFailed("send album request", response.message);
      return;
    }

    showCreated(
      "Album request",
      `"${selectedResult.title}" was sent to the admin notification queue.`,
    );
    onOpenChange(false);
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="max-h-[85vh] overflow-y-auto border-border bg-card sm:max-w-2xl"
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
          <DialogTitle className="text-foreground">Request Album</DialogTitle>
          <DialogDescription className="text-muted-foreground">
            Search Spotify, pick the exact album you want, and send the request to an admin.
          </DialogDescription>
        </DialogHeader>

        <SpotifyAlbumPicker
          confirmLabel="Send Request"
          initialTitle=""
          onConfirm={handleConfirm}
          renderResultMeta={result => {
            if (!result.already_in_library) {
              return null;
            }

            return (
              <p className="text-primary">
                This album is already in your library. Submitting will open the existing album page.
              </p>
            );
          }}
        />
      </DialogContent>
    </Dialog>
  );
}
