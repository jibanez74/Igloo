import { Play, Shuffle } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";

type PlayShuffleButtonsProps = {
  /** The Play button's visible words, e.g. "Play Album" or "Play All". */
  playLabel: string;
  /** A fuller accessible name when the visible words need context. */
  playAriaLabel?: string;
  shuffleAriaLabel: string;
  onPlay: () => void;
  onShuffle: () => void;
  /** Swaps both icons for a spinner while the rest of a queue loads. */
  isLoading?: boolean;
};

/**
 * The Play / Shuffle pair on the album, musician and playlist pages, for the
 * page's MUSIC_DETAIL_ACTIONS_CLASS row. Deliberately never disabled:
 * playback starts from what is already loaded, and a disabled media control is
 * unreachable under iOS VoiceOver. `size="lg"` re-declares rounded-md, so the
 * outline Shuffle re-asserts rounded-full to match the Play pill (§1.6).
 */
export default function PlayShuffleButtons({
  playLabel,
  playAriaLabel,
  shuffleAriaLabel,
  onPlay,
  onShuffle,
  isLoading = false,
}: PlayShuffleButtonsProps) {
  return (
    <>
      <Button
        type="button"
        variant="accent-pill"
        size="lg"
        onClick={onPlay}
        className="w-full font-semibold shadow-lg shadow-primary/20 sm:w-auto"
        aria-label={playAriaLabel}
      >
        {isLoading ? (
          <Spinner className="size-4" />
        ) : (
          <Play className="size-4 fill-current" aria-hidden="true" />
        )}
        {playLabel}
      </Button>
      <Button
        type="button"
        variant="outline"
        size="lg"
        onClick={onShuffle}
        className="w-full rounded-full font-semibold sm:w-auto"
        aria-label={shuffleAriaLabel}
      >
        {isLoading ? (
          <Spinner className="size-4" />
        ) : (
          <Shuffle className="size-4" aria-hidden="true" />
        )}
        Shuffle
      </Button>
    </>
  );
}
