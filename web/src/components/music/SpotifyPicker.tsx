import { useId, useState, type KeyboardEvent, type ReactNode } from "react";
import { Disc3, Music, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import {
  MOTION_MICRO_COLORS_CLASS,
  PEER_FOCUS_VISIBLE_RING_CLASS,
} from "@/lib/constants";
import { pluralize } from "@/lib/format";
import { showActionFailed, showInfo } from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import type { ApiResponseType } from "@/types";

/**
 * Deliberately not shared with `movies/TmdbMoviePicker`, which looks similar
 * and is not: it takes year and TMDB-id fields, marks results already in the
 * library as unselectable, and rethrows from `onConfirm` where this swallows
 * into a toast. Those are behaviors, not styling.
 */
export type SpotifyPickerKind = "album" | "track";

/** What the picker itself reads off a result; each kind carries more. */
export type SpotifySearchResult = {
  spotify_id: string;
  title: string;
  artist_names: string[];
  cover_url: string;
};

const KIND_COPY = {
  album: {
    label: "Album title",
    legend: "Spotify album results",
    empty: "No Spotify album matches found",
    icon: Disc3,
  },
  track: {
    label: "Track title",
    legend: "Spotify track results",
    empty: "No Spotify track matches found",
    icon: Music,
  },
} as const;

type SpotifyPickerProps<T extends SpotifySearchResult> = {
  kind: SpotifyPickerKind;
  confirmLabel: string;
  initialTitle: string;
  searchFn: (body: {
    title: string;
  }) => Promise<ApiResponseType<{ results: T[] }>>;
  onConfirm: (result: T) => Promise<void>;
  /** Parenthetical after the title: a release year, or a duration. */
  getTitleSuffix: (result: T) => string;
  /** Muted lines under the artists row, above the Spotify id. */
  renderDetails?: (result: T) => ReactNode;
  /** Extra content below the card, e.g. an already-in-library note. */
  renderResultMeta?: (result: T) => ReactNode;
};

export default function SpotifyPicker<T extends SpotifySearchResult>({
  kind,
  confirmLabel,
  initialTitle,
  searchFn,
  onConfirm,
  getTitleSuffix,
  renderDetails,
  renderResultMeta,
}: SpotifyPickerProps<T>) {
  const pickerId = useId().replace(/:/g, "");
  const [title, setTitle] = useState(initialTitle);
  const [results, setResults] = useState<T[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [searching, setSearching] = useState(false);
  const [confirming, setConfirming] = useState(false);

  const copy = KIND_COPY[kind];
  const trimmedTitle = title.trim();
  const selectedResult =
    results.find(result => result.spotify_id === selectedId) ?? null;
  const canSearch = trimmedTitle.length > 0;
  const titleInputId = `${pickerId}-title`;
  const resultsGroupName = `${pickerId}-spotify-${kind}-result`;
  const resultsLabelId = `${pickerId}-spotify-results-label`;

  function getResultInputId(spotifyId: string) {
    return `${pickerId}-spotify-${kind}-result-${spotifyId}`;
  }

  function handleResultArrowKey(
    event: KeyboardEvent<HTMLInputElement>,
    currentIndex: number,
  ) {
    if (
      event.key === " "
      || event.key === "Space"
      || event.key === "Spacebar"
      || event.code === "Space"
    ) {
      event.preventDefault();
      setSelectedId(results[currentIndex].spotify_id);
      return;
    }

    if (results.length < 2) {
      return;
    }

    let nextIndex = currentIndex;

    if (event.key === "ArrowDown" || event.key === "ArrowRight") {
      nextIndex = (currentIndex + 1) % results.length;
    } else if (event.key === "ArrowUp" || event.key === "ArrowLeft") {
      nextIndex = (currentIndex - 1 + results.length) % results.length;
    } else {
      return;
    }

    event.preventDefault();

    const nextResult = results[nextIndex];
    const nextInput = document.getElementById(getResultInputId(nextResult.spotify_id));
    if (nextInput instanceof HTMLInputElement) {
      nextInput.focus();
    }

    setSelectedId(nextResult.spotify_id);
  }

  async function handleSearch() {
    if (!canSearch) return;

    setSearching(true);
    setResults([]);
    setSelectedId(null);

    try {
      const response = await searchFn({
        title: trimmedTitle,
      });

      if (response.error || !response.data?.results) {
        showActionFailed("search Spotify", response.message);
      } else {
        setResults(response.data.results);
        if (response.data.results.length === 0) {
          showInfo(copy.empty);
        }
      }
    } catch {
      showActionFailed(
        "search Spotify",
        "Unable to complete Spotify search right now.",
      );
    }

    setSearching(false);
  }

  async function handleConfirm() {
    if (selectedResult == null) return;

    setConfirming(true);
    try {
      await onConfirm(selectedResult);
    } catch {
      showActionFailed("send request", "Unable to complete this action right now.");
    }

    setConfirming(false);
  }

  return (
    <div className="space-y-4">
      <div>
        <Label htmlFor={titleInputId} className="text-muted-foreground">
          {copy.label}
        </Label>
        <Input
          id={titleInputId}
          value={title}
          onChange={event => setTitle(event.target.value)}
          className="mt-1 border-border bg-muted text-foreground"
          autoComplete="off"
          autoFocus
        />
      </div>

      <Button
        onClick={handleSearch}
        disabled={searching || !canSearch}
        className="w-full"
      >
        {searching ? (
          <Spinner className="size-4" />
        ) : (
          <Search className="size-4" aria-hidden="true" />
        )}
        {searching ? "Searching..." : "Search Spotify"}
      </Button>

      {results.length > 0 && (
        <fieldset className="space-y-2">
          <legend id={resultsLabelId} className="sr-only">
            {copy.legend}
          </legend>
          <p className="text-sm text-muted-foreground">
            {pluralize(results.length, "result")} found
          </p>

          <ul className="max-h-72 space-y-2 overflow-y-auto">
            {results.map((result, index) => {
              const meta = renderResultMeta?.(result);

              return (
                <SpotifyResultCard
                  key={result.spotify_id}
                  details={renderDetails?.(result)}
                  fallbackIcon={copy.icon}
                  inputId={getResultInputId(result.spotify_id)}
                  meta={meta}
                  metaId={meta ? `${pickerId}-spotify-meta-${result.spotify_id}` : undefined}
                  name={resultsGroupName}
                  onKeyDown={event => handleResultArrowKey(event, index)}
                  onSelect={() => setSelectedId(result.spotify_id)}
                  result={result}
                  selected={selectedId === result.spotify_id}
                  titleSuffix={getTitleSuffix(result)}
                />
              );
            })}
          </ul>
        </fieldset>
      )}

      <Button
        onClick={handleConfirm}
        disabled={selectedResult == null || confirming}
        variant="accent"
        className="w-full"
      >
        {confirming && <Spinner className="size-4" />}
        {confirming ? `${confirmLabel}...` : confirmLabel}
      </Button>
    </div>
  );
}

function SpotifyResultCard({
  details,
  fallbackIcon: FallbackIcon,
  inputId,
  meta,
  metaId,
  name,
  onKeyDown,
  onSelect,
  result,
  selected,
  titleSuffix,
}: {
  details?: ReactNode;
  fallbackIcon: typeof Disc3;
  inputId: string;
  meta?: ReactNode;
  metaId?: string;
  name: string;
  onKeyDown: (event: KeyboardEvent<HTMLInputElement>) => void;
  onSelect: () => void;
  result: SpotifySearchResult;
  selected: boolean;
  titleSuffix: string;
}) {
  const artistsLabel =
    result.artist_names.length > 0
      ? result.artist_names.join(", ")
      : "Unknown artist";
  const labelId = `${inputId}-label`;

  return (
    <li
      className={cn(
        MOTION_MICRO_COLORS_CLASS,
        "overflow-hidden rounded-lg border",
        selected
          ? "border-primary bg-primary/10"
          : "border-border bg-muted/60 hover:border-border",
      )}
    >
      <input
        id={inputId}
        type="radio"
        name={name}
        checked={selected}
        onChange={onSelect}
        onKeyDown={onKeyDown}
        aria-labelledby={labelId}
        aria-describedby={metaId}
        className="peer sr-only"
      />
      <Label
        id={labelId}
        htmlFor={inputId}
        className={cn(
          "mb-0 flex cursor-pointer gap-3 rounded-lg p-2",
          PEER_FOCUS_VISIBLE_RING_CLASS,
        )}
      >
        {result.cover_url ? (
          <img
            src={result.cover_url}
            alt=""
            className="size-20 shrink-0 rounded-sm object-cover"
          />
        ) : (
          <div className="flex size-20 shrink-0 items-center justify-center rounded-sm bg-accent">
            <FallbackIcon
              className="size-6 text-muted-foreground"
              aria-hidden="true"
            />
          </div>
        )}
        <div className="min-w-0 flex-1">
          <p className="truncate font-medium text-foreground">
            {result.title}
            {titleSuffix && (
              <span className="ml-1 text-muted-foreground">({titleSuffix})</span>
            )}
          </p>
          <p className="mt-0.5 truncate text-sm text-muted-foreground">
            {artistsLabel}
          </p>
          {details}
          <p className="mt-1 text-xs text-muted-foreground">
            Spotify ID: {result.spotify_id}
          </p>
        </div>
      </Label>
      {meta ? <div id={metaId} className="px-2 pb-2 pl-21 text-sm">{meta}</div> : null}
    </li>
  );
}
