import { useState, type ReactNode } from "react";
import { Disc3, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { searchSpotifyAlbums } from "@/lib/api";
import { showActionFailed, showInfo } from "@/lib/toast-helpers";
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
  const [title, setTitle] = useState(initialTitle);
  const [results, setResults] = useState<SpotifyAlbumSearchResultType[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [searching, setSearching] = useState(false);
  const [confirming, setConfirming] = useState(false);

  const trimmedTitle = title.trim();
  const selectedResult =
    results.find(result => result.spotify_id === selectedId) ?? null;
  const canSearch = trimmedTitle.length > 0;

  async function handleSearch() {
    if (!canSearch) return;

    setSearching(true);
    setResults([]);
    setSelectedId(null);

    const response = await searchFn({
      title: trimmedTitle,
    });
    setSearching(false);

    if (response.error || !response.data?.results) {
      showActionFailed("search Spotify", response.message);
      return;
    }

    setResults(response.data.results);
    if (response.data.results.length === 0) {
      showInfo("No Spotify album matches found");
    }
  }

  async function handleConfirm() {
    if (selectedResult == null) return;

    setConfirming(true);
    try {
      await onConfirm(selectedResult);
    } finally {
      setConfirming(false);
    }
  }

  return (
    <div className="space-y-4">
      <div>
        <Label htmlFor="spotify-album-title" className="text-slate-300">
          Album title
        </Label>
        <Input
          id="spotify-album-title"
          value={title}
          onChange={event => setTitle(event.target.value)}
          className="mt-1 border-slate-700 bg-slate-800 text-white"
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
        <div className="space-y-2">
          <p className="text-sm text-slate-400">
            {results.length} result{results.length === 1 ? "" : "s"} found
          </p>

          <ul
            className="max-h-72 space-y-2 overflow-y-auto"
            role="listbox"
            aria-label="Spotify album results"
          >
            {results.map(result => (
              <SpotifyAlbumResultCard
                key={result.spotify_id}
                result={result}
                selected={selectedId === result.spotify_id}
                onSelect={() => setSelectedId(result.spotify_id)}
                meta={renderResultMeta?.(result)}
              />
            ))}
          </ul>
        </div>
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
  meta,
  onSelect,
  result,
  selected,
}: {
  meta?: ReactNode;
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

  return (
    <li
      role="option"
      aria-selected={selected}
      tabIndex={0}
      onClick={onSelect}
      onKeyDown={event => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onSelect();
        }
      }}
      className={`flex cursor-pointer gap-3 rounded-lg border p-2 transition-colors outline-none focus-visible:ring-2 focus-visible:ring-amber-500/60 ${
        selected
          ? "border-amber-500 bg-amber-500/10"
          : "border-slate-700 bg-slate-800/60 hover:border-slate-600"
      }`}
    >
      {result.cover_url ? (
        <img
          src={result.cover_url}
          alt=""
          className="size-20 shrink-0 rounded-sm object-cover"
        />
      ) : (
        <div className="flex size-20 shrink-0 items-center justify-center rounded-sm bg-slate-700">
          <Disc3 className="size-6 text-slate-500" aria-hidden="true" />
        </div>
      )}
      <div className="min-w-0 flex-1">
        <p className="truncate font-medium text-white">
          {result.title}
          {releaseYear && (
            <span className="ml-1 text-slate-400">({releaseYear})</span>
          )}
        </p>
        <p className="mt-0.5 truncate text-sm text-slate-400">
          {artistsLabel}
        </p>
        <p className="mt-1 text-xs text-slate-500">
          Spotify ID: {result.spotify_id}
        </p>
        <p className="mt-0.5 text-xs text-slate-500">
          {albumType}
          {result.total_tracks > 0
            ? ` - ${result.total_tracks} track${result.total_tracks === 1 ? "" : "s"}`
            : ""}
        </p>
        {meta ? <div className="mt-2 text-sm">{meta}</div> : null}
      </div>
    </li>
  );
}
