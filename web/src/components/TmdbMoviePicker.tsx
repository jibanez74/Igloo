import { useId, useState, type KeyboardEvent, type ReactNode } from "react";
import { Film, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { searchTmdbMovies } from "@/lib/api";
import { TMDB_POSTER_SIZE } from "@/lib/constants";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { showActionFailed, showInfo } from "@/lib/toast-helpers";
import { cn } from "@/lib/utils";
import type { ApiResponseType, TmdbSearchMoviesRequest, TmdbSearchResultType } from "@/types";

type TmdbMoviePickerProps = {
  confirmLabel: string;
  confirmVariant?: "default" | "accent";
  initialTitle: string;
  initialTmdbId?: string;
  initialYear?: string;
  isResultBlocked?: (result: TmdbSearchResultType) => boolean;
  onConfirm: (result: TmdbSearchResultType) => Promise<void>;
  renderResultMeta?: (result: TmdbSearchResultType) => ReactNode;
  searchFn?: (
    body: TmdbSearchMoviesRequest,
  ) => Promise<ApiResponseType<{ results: TmdbSearchResultType[] }>>;
  showTmdbIdInput?: boolean;
};

export default function TmdbMoviePicker({
  confirmLabel,
  confirmVariant = "accent",
  initialTitle,
  initialTmdbId = "",
  initialYear = "",
  isResultBlocked,
  onConfirm,
  renderResultMeta,
  searchFn = searchTmdbMovies,
  showTmdbIdInput = false,
}: TmdbMoviePickerProps) {
  const pickerId = useId().replace(/:/g, "");
  const [title, setTitle] = useState(initialTitle);
  const [year, setYear] = useState(initialYear);
  const [tmdbIdInput, setTmdbIdInput] = useState(initialTmdbId);
  const [results, setResults] = useState<TmdbSearchResultType[]>([]);
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [searching, setSearching] = useState(false);
  const [confirming, setConfirming] = useState(false);

  const selectedResult = results.find(result => result.tmdb_id === selectedId) ?? null;
  const isSelectedBlocked =
    selectedResult != null && isResultBlocked
      ? isResultBlocked(selectedResult)
      : false;
  const canSearch = title.trim().length > 0 || tmdbIdInput.trim().length > 0;
  const resultsGroupName = `${pickerId}-tmdb-result`;
  const resultsLabelId = `${pickerId}-tmdb-results-label`;

  function getResultInputId(tmdbId: number) {
    return `${pickerId}-tmdb-result-${tmdbId}`;
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
      setSelectedId(results[currentIndex].tmdb_id);
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
    const nextInput = document.getElementById(getResultInputId(nextResult.tmdb_id));
    if (nextInput instanceof HTMLInputElement) {
      nextInput.focus();
    }

    setSelectedId(nextResult.tmdb_id);
  }

  async function handleSearch() {
    if (!canSearch) return;

    setSearching(true);
    setResults([]);
    setSelectedId(null);

    const body: TmdbSearchMoviesRequest = {
      title: title.trim(),
    };
    const parsedYear = Number.parseInt(year, 10);
    if (parsedYear > 0) {
      body.year = parsedYear;
    }
    const parsedTmdbId = Number.parseInt(tmdbIdInput, 10);
    if (parsedTmdbId > 0) {
      body.tmdb_id = parsedTmdbId;
    }

    const response = await searchFn(body);
    setSearching(false);

    if (response.error || !response.data?.results) {
      showActionFailed("search TMDB", response.message);
      return;
    }

    setResults(response.data.results);
    if (response.data.results.length === 0) {
      showInfo("No TMDB matches found");
    }
  }

  async function handleConfirm() {
    if (selectedResult == null || isSelectedBlocked) return;

    setConfirming(true);
    let caughtError: unknown;
    let didFail = false;

    try {
      await onConfirm(selectedResult);
    } catch (error) {
      caughtError = error;
      didFail = true;
    }

    setConfirming(false);
    if (didFail) {
      throw caughtError;
    }
  }

  return (
    <div className="space-y-4">
      <div className={`grid gap-3 ${showTmdbIdInput ? "sm:grid-cols-3" : "sm:grid-cols-2"}`}>
        <div className={showTmdbIdInput ? "sm:col-span-1" : ""}>
          <Label htmlFor="tmdb-title" className="text-slate-300">
            Title
          </Label>
          <Input
            id="tmdb-title"
            value={title}
            onChange={event => setTitle(event.target.value)}
            className="mt-1 border-slate-700 bg-slate-800 text-white"
            autoComplete="off"
            autoFocus
          />
        </div>
        <div>
          <Label htmlFor="tmdb-year" className="text-slate-300">
            Year
          </Label>
          <Input
            id="tmdb-year"
            type="number"
            value={year}
            onChange={event => setYear(event.target.value)}
            className="mt-1 border-slate-700 bg-slate-800 text-white"
            inputMode="numeric"
          />
        </div>
        {showTmdbIdInput && (
          <div>
            <Label htmlFor="tmdb-id" className="text-slate-300">
              TMDB ID
            </Label>
            <Input
              id="tmdb-id"
              type="number"
              value={tmdbIdInput}
              onChange={event => setTmdbIdInput(event.target.value)}
              placeholder="Optional"
              className="mt-1 border-slate-700 bg-slate-800 text-white"
              inputMode="numeric"
            />
          </div>
        )}
      </div>

      <Button onClick={handleSearch} disabled={searching || !canSearch} className="w-full">
        {searching ? (
          <Spinner className="size-4" />
        ) : (
          <Search className="size-4" aria-hidden="true" />
        )}
        {searching ? "Searching…" : "Search TMDB"}
      </Button>

      {results.length > 0 && (
        <fieldset className="space-y-2">
          <legend id={resultsLabelId} className="sr-only">
            TMDB movie results
          </legend>
          <p className="text-sm text-slate-400">
            {results.length} result{results.length === 1 ? "" : "s"} found
          </p>

          <ul className="max-h-64 space-y-2 overflow-y-auto">
            {results.map((result, index) => {
              const meta = renderResultMeta?.(result);

              return (
                <TmdbResultCard
                  key={result.tmdb_id}
                  inputId={getResultInputId(result.tmdb_id)}
                  meta={meta}
                  metaId={meta ? `${pickerId}-tmdb-meta-${result.tmdb_id}` : undefined}
                  name={resultsGroupName}
                  onKeyDown={event => handleResultArrowKey(event, index)}
                  onSelect={() => setSelectedId(result.tmdb_id)}
                  result={result}
                  selected={selectedId === result.tmdb_id}
                />
              );
            })}
          </ul>
        </fieldset>
      )}

      <Button
        onClick={handleConfirm}
        disabled={selectedResult == null || confirming || isSelectedBlocked}
        variant={confirmVariant}
        className="w-full"
      >
        {confirming && <Spinner className="size-4" />}
        {confirming ? `${confirmLabel}…` : confirmLabel}
      </Button>
    </div>
  );
}

function TmdbResultCard({
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
  result: TmdbSearchResultType;
  selected: boolean;
}) {
  const posterUrl = result.poster_path
    ? buildTmdbImageUrl(result.poster_path, TMDB_POSTER_SIZE)
    : null;
  const releaseYear = result.release_date?.slice(0, 4);
  const labelId = `${inputId}-label`;

  return (
    <li
      className={cn(
        "overflow-hidden rounded-lg border transition-colors",
        selected
          ? "border-amber-500 bg-amber-500/10"
          : "border-slate-700 bg-slate-800/60 hover:border-slate-600",
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
        {posterUrl ? (
          <img
            src={posterUrl}
            alt=""
            className="h-20 w-14 shrink-0 rounded-sm object-cover"
          />
        ) : (
          <div className="flex h-20 w-14 shrink-0 items-center justify-center rounded-sm bg-slate-700">
            <Film className="size-5 text-slate-500" aria-hidden="true" />
          </div>
        )}
        <div className="min-w-0 flex-1">
          <p className="truncate font-medium text-white">
            {result.title}
            {releaseYear && (
              <span className="ml-1 text-slate-400">({releaseYear})</span>
            )}
          </p>
          <p className="mt-0.5 text-xs text-slate-500">
            TMDB ID: {result.tmdb_id}
          </p>
          {result.overview && (
            <p className="mt-1 line-clamp-2 text-sm text-slate-400">
              {result.overview}
            </p>
          )}
        </div>
      </Label>
      {meta ? <div id={metaId} className="px-2 pb-2 pl-19 text-sm">{meta}</div> : null}
    </li>
  );
}
