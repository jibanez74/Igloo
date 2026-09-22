import type { ReactNode, RefObject } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import SpotifyPicker, {
  type SpotifyPickerKind,
  type SpotifySearchResult,
} from "@/components/music/SpotifyPicker";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import { createNotification } from "@/lib/api";
import { NOTIFICATION_TITLES } from "@/lib/constants";
import { authUserQueryOpts } from "@/lib/query-opts";
import { showActionFailed, showCreated } from "@/lib/toast-helpers";
import type { ApiResponseType } from "@/types";

const KIND_COPY = {
  album: {
    heading: "Request Album",
    description:
      "Search Spotify, pick the exact album you want, and send the request to an admin.",
    notificationTitle: NOTIFICATION_TITLES.ALBUM_REQUEST,
    toastTitle: "Album request",
    failureAction: "send album request",
    spotifyPath: "album",
  },
  track: {
    heading: "Request Track",
    description:
      "Search Spotify, pick the exact track you want, and send the request to an admin.",
    notificationTitle: NOTIFICATION_TITLES.TRACK_REQUEST,
    toastTitle: "Track request",
    failureAction: "send track request",
    spotifyPath: "track",
  },
} as const;

type SpotifyRequestDialogProps<T extends SpotifySearchResult> = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  restoreFocusRef?: RefObject<HTMLElement | null>;
  kind: SpotifyPickerKind;
  searchFn: (body: {
    title: string;
  }) => Promise<ApiResponseType<{ results: T[] }>>;
  getTitleSuffix: (result: T) => string;
  renderDetails?: (result: T) => ReactNode;
  renderResultMeta?: (result: T) => ReactNode;
  /**
   * The lines describing the thing itself, between the requester and the
   * Spotify id — falsy entries are dropped.
   */
  buildDetailLines: (result: T) => (string | null)[];
  /**
   * A chance to handle the result without sending a request; return true when
   * it did. Albums already in the library open instead of being requested.
   */
  interceptConfirm?: (result: T) => boolean;
};

/**
 * "Request Album" / "Request Track": search Spotify, pick one result, and file
 * it in the admin notification queue. The two differ only in what they call the
 * thing and which lines describe it.
 */
export default function SpotifyRequestDialog<T extends SpotifySearchResult>({
  open,
  onOpenChange,
  restoreFocusRef,
  kind,
  searchFn,
  getTitleSuffix,
  renderDetails,
  renderResultMeta,
  buildDetailLines,
  interceptConfirm,
}: SpotifyRequestDialogProps<T>) {
  const { data: authData } = useQuery(authUserQueryOpts());
  const copy = KIND_COPY[kind];

  async function handleConfirm(selectedResult: T) {
    if (interceptConfirm?.(selectedResult)) return;

    if (authData?.error !== false) {
      showActionFailed(
        copy.failureAction,
        "Your account details are unavailable right now.",
      );
      return;
    }

    const requester = authData.data.user;
    const spotifyURL =
      ("spotify_url" in selectedResult
        && (selectedResult as { spotify_url?: string }).spotify_url)
      || `https://open.spotify.com/${copy.spotifyPath}/${selectedResult.spotify_id}`;

    const lines = [
      `Requester: ${requester.name} <${requester.email}>`,
      ...buildDetailLines(selectedResult),
      `Spotify ID: ${selectedResult.spotify_id}`,
      `Spotify URL: ${spotifyURL}`,
    ].filter(Boolean);

    const response = await createNotification({
      title: copy.notificationTitle,
      message: lines.join("\n"),
      isAdmin: true,
    });

    if (response.error) {
      showActionFailed(copy.failureAction, response.message);
      return;
    }

    showCreated(
      copy.toastTitle,
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
          <DialogTitle className="text-foreground">{copy.heading}</DialogTitle>
          <DialogDescription className="text-muted-foreground">
            {copy.description}
          </DialogDescription>
        </DialogHeader>

        <SpotifyPicker
          kind={kind}
          confirmLabel="Send Request"
          initialTitle=""
          searchFn={searchFn}
          onConfirm={handleConfirm}
          getTitleSuffix={getTitleSuffix}
          renderDetails={renderDetails}
          renderResultMeta={renderResultMeta}
        />
      </DialogContent>
    </Dialog>
  );
}
