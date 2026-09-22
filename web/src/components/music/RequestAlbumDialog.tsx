import type { RefObject } from "react";
import { useNavigate } from "@tanstack/react-router";
import SpotifyRequestDialog from "@/components/music/SpotifyRequestDialog";
import { searchSpotifyAlbums } from "@/lib/api";
import { pluralize } from "@/lib/format";
import { showActionFailed } from "@/lib/toast-helpers";
import type { SpotifyAlbumSearchResultType } from "@/types";

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

  // An album the library already has is not worth an admin's attention, so
  // picking it opens the album instead of filing a request.
  function openExistingAlbum(result: SpotifyAlbumSearchResultType) {
    if (!result.already_in_library) return false;

    if (result.library_album_id == null) {
      showActionFailed(
        "open existing album",
        "The matching album exists, but its page could not be identified.",
      );
      return true;
    }

    onOpenChange(false);
    void navigate({
      to: "/music/album/$id",
      params: { id: String(result.library_album_id) },
    });
    return true;
  }

  return (
    <SpotifyRequestDialog
      open={open}
      onOpenChange={onOpenChange}
      restoreFocusRef={restoreFocusRef}
      kind="album"
      searchFn={searchSpotifyAlbums}
      getTitleSuffix={result => result.release_date.slice(0, 4)}
      renderDetails={result => (
        <p className="mt-1 text-xs text-muted-foreground">
          {result.album_type || "album"}
          {result.total_tracks > 0
            ? ` - ${pluralize(result.total_tracks, "track")}`
            : ""}
        </p>
      )}
      renderResultMeta={result =>
        result.already_in_library ? (
          <p className="text-primary">
            This album is already in your library. Submitting will open the existing album page.
          </p>
        ) : null
      }
      buildDetailLines={result => [
        `Album: ${result.title}`,
        result.artist_names.length > 0
          ? `Artists: ${result.artist_names.join(", ")}`
          : null,
        result.release_date ? `Release date: ${result.release_date}` : null,
        result.total_tracks > 0 ? `Total tracks: ${result.total_tracks}` : null,
      ]}
      interceptConfirm={openExistingAlbum}
    />
  );
}
