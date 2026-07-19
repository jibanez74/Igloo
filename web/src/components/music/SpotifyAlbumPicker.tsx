import { useId, useState, type KeyboardEvent, type ReactNode } from "react";
import { Disc3, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { searchSpotifyAlbums } from "@/lib/api";
import { MOTION_MICRO_COLORS_CLASS } from "@/lib/constants";
import { showActionFailed, showInfo } from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import type {
  ApiResponseType,
  SpotifyAlbumSearchRequest,
  SpotifyAlbumSearchResultType,
} from "@/types";

type SpotifyAlbumPickerProps = {
  confirmLabel: string;
  initialTitle: string;
  onConfirm: (result: SpotifyAlbumSearchResultType) => Promise<void>;
  renderResultMeta?: (result: SpotifyAlbumSearchResultType) => ReactNode;
  searchFn?: (
    body: SpotifyAlbumSearchRequest,
  ) => Promise<ApiResponseType<{ results: SpotifyAlbumSearchResultType[] }>>;
};

export default function SpotifyAlbumPicker({
  confirmLabel,
  initialTitle,
  onConfirm,
  renderResultMeta,
  searchFn = searchSpotifyAlbums,
}: SpotifyAlbumPickerProps) {
  const pickerId = useId().replace(/:/g, "");
  const [title, setTitle] = useState(initialTitle);
  const [results, setResults] = useState<SpotifyAlbumSearchResultType[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [searching, setSearching] = useState(false);
  const [confirming, setConfirming] = useState(false);

  const trimmedTitle = title.trim();
  const selectedResult =
    results.find(result => result.spotify_id === selectedId) ?? null;
  const canSearch = trimmedTitle.length > 0;
  const resultsGroupName = `${pickerId}-spotify-album-result`;
  const resultsLabelId = `${pickerId}-spotify-results-label`;

  function getResultInputId(spotifyId: string) {
    return `${pickerId}-spotify-album-result-${spotifyId}`;
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
          showInfo("No Spotify album matches found");
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
        <Label htmlFor="spotify-album-title" className="text-muted-foreground">
          Album title
        </Label>
        <Input
          id="spotify-album-title"
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
            Spotify album results
          </legend>
          <p className="text-sm text-muted-foreground">
            {results.length} result{results.length === 1 ? "" : "s"} found
          </p>

          <ul className="max-h-72 space-y-2 overflow-y-auto">
            {results.map((result, index) => {
              const meta = renderResultMeta?.(result);

              return (
                <SpotifyAlbumResultCard
                  key={result.spotify_id}
                  inputId={getResultInputId(result.spotify_id)}
                  meta={meta}
                  metaId={meta ? `${pickerId}-spotify-meta-${result.spotify_id}` : undefined}
                  name={resultsGroupName}
                  onKeyDown={event => handleResultArrowKey(event, index)}
                  onSelect={() => setSelectedId(result.spotify_id)}
                  result={result}
                  selected={selectedId === result.spotify_id}
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

function SpotifyAlbumResultCard({
  inputId,
  meta,
  metaId,
  name,
  onKeyDown,
  onSelect,
  result,
  selected,
}: {
  inputId: string;
  meta?: ReactNode;
  metaId?: string;
  name: string;
  onKeyDown: (event: KeyboardEvent<HTMLInputElement>) => void;
  onSelect: () => void;
  result: SpotifyAlbumSearchResultType;
  selected: boolean;
}) {
  const artistsLabel =
    result.artist_names.length > 0
      ? result.artist_names.join(", ")
      : "Unknown artist";
  const releaseYear = result.release_date.slice(0, 4);
  const albumType = result.album_type ? result.album_type : "album";
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
        className="mb-0 flex cursor-pointer gap-3 rounded-lg p-2 peer-focus-visible:ring-2 peer-focus-visible:ring-ring/60"
      >
        {result.cover_url ? (
          <img
            src={result.cover_url}
            alt=""
            className="size-20 shrink-0 rounded-sm object-cover"
          />
        ) : (
          <div className="flex size-20 shrink-0 items-center justify-center rounded-sm bg-accent">
            <Disc3 className="size-6 text-muted-foreground" aria-hidden="true" />
          </div>
        )}
        <div className="min-w-0 flex-1">
          <p className="truncate font-medium text-foreground">
            {result.title}
            {releaseYear && (
              <span className="ml-1 text-muted-foreground">({releaseYear})</span>
            )}
          </p>
          <p className="mt-0.5 truncate text-sm text-muted-foreground">
            {artistsLabel}
          </p>
          <p className="mt-1 text-xs text-muted-foreground">
            Spotify ID: {result.spotify_id}
          </p>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {albumType}
            {result.total_tracks > 0
              ? ` - ${result.total_tracks} track${result.total_tracks === 1 ? "" : "s"}`
              : ""}
          </p>
        </div>
      </Label>
      {meta ? <div id={metaId} className="px-2 pb-2 pl-21 text-sm">{meta}</div> : null}
    </li>
  );
}
