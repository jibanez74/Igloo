import type { RefObject } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import SpotifyTrackPicker from "@/components/SpotifyTrackPicker";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import { createNotification } from "@/lib/api";
import { authUserQueryOpts } from "@/lib/query-opts";
import {
  showActionFailed,
  showCreated,
} from "@/lib/toast-helpers";
import type { SpotifyTrackSearchResultType } from "@/types";

const TRACK_REQUEST_NOTIFICATION_TITLE = "track_request";

type RequestTrackDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  restoreFocusRef?: RefObject<HTMLElement | null>;
};

export default function RequestTrackDialog({
  open,
  onOpenChange,
  restoreFocusRef,
}: RequestTrackDialogProps) {
  const { data: authData } = useQuery(authUserQueryOpts());

  async function handleConfirm(selectedResult: SpotifyTrackSearchResultType) {
    if (authData?.error !== false) {
      showActionFailed(
        "send track request",
        "Your account details are unavailable right now.",
      );
      return;
    }

    const requester = authData.data.user;
    const spotifyURL =
      selectedResult.spotify_url ||
      `https://open.spotify.com/track/${selectedResult.spotify_id}`;
    const artists = selectedResult.artist_names.join(", ");
    const lines = [
      `Requester: ${requester.name}`,
      `Track: ${selectedResult.title}`,
      artists ? `Artists: ${artists}` : null,
      selectedResult.album_name ? `Album: ${selectedResult.album_name}` : null,
      `Spotify ID: ${selectedResult.spotify_id}`,
      `Spotify URL: ${spotifyURL}`,
    ].filter(Boolean);

    const response = await createNotification({
      title: TRACK_REQUEST_NOTIFICATION_TITLE,
      message: lines.join("\n"),
      isAdmin: true,
    });

    if (response.error) {
      showActionFailed("send track request", response.message);
      return;
    }

    showCreated(
      "Track request",
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
          <DialogTitle className="text-foreground">Request Track</DialogTitle>
          <DialogDescription className="text-muted-foreground">
            Search Spotify, pick the exact track you want, and send the request to an admin.
          </DialogDescription>
        </DialogHeader>

        <SpotifyTrackPicker
          confirmLabel="Send Request"
          initialTitle=""
          onConfirm={handleConfirm}
        />
      </DialogContent>
    </Dialog>
  );
}
