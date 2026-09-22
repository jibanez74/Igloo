import type { RefObject } from "react";
import SpotifyRequestDialog from "@/components/music/SpotifyRequestDialog";
import { searchSpotifyTracks } from "@/lib/api";
import { formatTrackDuration } from "@/lib/format";
import type { SpotifyTrackSearchResultType } from "@/types";

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
  return (
    <SpotifyRequestDialog<SpotifyTrackSearchResultType>
      open={open}
      onOpenChange={onOpenChange}
      restoreFocusRef={restoreFocusRef}
      kind="track"
      searchFn={searchSpotifyTracks}
      getTitleSuffix={result => formatTrackDuration(result.duration_ms)}
      renderDetails={result =>
        result.album_name ? (
          <p className="mt-1 truncate text-xs text-muted-foreground">
            Album: {result.album_name}
          </p>
        ) : null
      }
      buildDetailLines={result => [
        `Track: ${result.title}`,
        result.artist_names.length > 0
          ? `Artists: ${result.artist_names.join(", ")}`
          : null,
        result.album_name ? `Album: ${result.album_name}` : null,
      ]}
    />
  );
}
