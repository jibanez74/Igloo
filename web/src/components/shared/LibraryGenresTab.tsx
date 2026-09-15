import { Fragment, useEffect, useRef, type ReactNode, type RefObject } from "react";
import { useQuery, type QueryKey } from "@tanstack/react-query";
import { X, type LucideIcon } from "lucide-react";
import {
  LibraryAllTabSkeleton,
  type LibraryNoun,
  type LibraryPageData,
  type LibraryQueryOptions,
} from "@/components/shared/LibraryAllTab";
import LibraryEmptyState from "@/components/shared/LibraryEmptyState";
import LibraryPagination from "@/components/shared/LibraryPagination";
import LibrarySortToggle, {
  type LibrarySortDirection,
} from "@/components/shared/LibrarySortToggle";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import { MoviesLoadError } from "@/components/shared/MoviesLoadError";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import {
  FOCUS_VISIBLE_RING_CLASS,
  LIBRARY_POSTER_GRID_CLASS,
  MOTION_LOADING_STATE_CLASS,
  MOTION_MICRO_CONTROL_CLASS,
} from "@/lib/constants";
import { isApiFailure } from "@/lib/is-api-failure";
import { scrollWindowToTop } from "@/lib/motion";
import { cn } from "@/lib/utils";

/** One genre chip: the page maps its API row (movie_count, show_count, …) onto `count`. */
export type LibraryGenre = {
  genre_id: number;
  genre_tag: string;
  count: number;
};

type LibraryGenresTabProps<
  TItem extends { id: number },
  TData extends LibraryPageData & Record<string, unknown>,
  TGenresData extends Record<string, unknown>,
  TKey extends QueryKey,
  TGenresKey extends QueryKey,
> = {
  genresQueryOpts: LibraryQueryOptions<TGenresData, TGenresKey>;
  getGenres: (data: TGenresData) => LibraryGenre[];
  /** Accessible name of the genre list, e.g. "Movie genres". */
  genresListLabel: string;
  /** The selected genre's items; disabled by the page while no genre is selected. */
  itemsQueryOpts: LibraryQueryOptions<TData, TKey>;
  getItems: (data: TData) => TItem[];
  renderCard: (item: TItem) => ReactNode;
  genreId: number | undefined;
  genresPage: number;
  sort: LibrarySortDirection;
  perPage: number;
  noun: LibraryNoun;
  emptyIcon: LucideIcon;
  /** Where focus lands after "Clear" if the genre's own button is gone: the tab trigger. */
  fallbackFocusRef: RefObject<HTMLButtonElement | null>;
  onSelectGenre: (genreId: number) => void;
  onClearGenre: () => void;
  onPageChange: (page: number) => void;
  onSortToggle: () => void;
};

// The genre facet of a library page: a grid of genre chips, and once one is
// pressed, a paginated grid of its items beneath. Clearing the filter returns
// focus to the chip that was selected, so keyboard users are not dropped at
// the top of the document.
export default function LibraryGenresTab<
  TItem extends { id: number },
  TData extends LibraryPageData & Record<string, unknown>,
  TGenresData extends Record<string, unknown>,
  TKey extends QueryKey,
  TGenresKey extends QueryKey,
>({
  genresQueryOpts,
  getGenres,
  genresListLabel,
  itemsQueryOpts,
  getItems,
  renderCard,
  genreId,
  genresPage,
  sort,
  perPage,
  noun,
  emptyIcon,
  fallbackFocusRef,
  onSelectGenre,
  onClearGenre,
  onPageChange,
  onSortToggle,
}: LibraryGenresTabProps<TItem, TData, TGenresData, TKey, TGenresKey>) {
  const genreButtonRefs = useRef<Map<number, HTMLButtonElement> | null>(null);
  const pendingRestoreGenreIdRef = useRef<number | null>(null);

  const {
    data: genresRes,
    isError: genresError,
    isLoading: genresLoading,
    refetch: refetchGenres,
  } = useQuery(genresQueryOpts);

  const genres = genresRes?.error === false ? getGenres(genresRes.data) : [];

  const {
    data: itemsRes,
    isError: itemsError,
    isLoading: itemsLoading,
    refetch: refetchItems,
  } = useQuery(itemsQueryOpts);

  const items = itemsRes?.error === false ? getItems(itemsRes.data) : [];
  const totalPages = itemsRes?.error === false ? itemsRes.data.total_pages : 0;
  const total = itemsRes?.error === false ? itemsRes.data.total : 0;
  const hasMultiplePages = totalPages > 1;
  const hasSelectedGenre = genreId != null;

  const selectedGenreTag =
    genreId != null
      ? genres.find(g => g.genre_id === genreId)?.genre_tag
      : undefined;

  useEffect(() => {
    const restoreGenreId = pendingRestoreGenreIdRef.current;
    if (genreId != null || restoreGenreId == null) return;

    pendingRestoreGenreIdRef.current = null;
    focusDialogRestoreTarget(
      genreButtonRefs.current?.get(restoreGenreId),
      fallbackFocusRef.current,
    );
  }, [fallbackFocusRef, genreId]);

  // Same guard as LibraryAllTab: the API does not clamp the page, so an
  // out-of-range genresPage answers with no items but a positive total, and the
  // empty state below carries no pagination to escape from.
  useEffect(() => {
    if (totalPages > 0 && genresPage > totalPages) {
      onPageChange(totalPages);
    }
  }, [genresPage, totalPages, onPageChange]);

  const getAnnouncement = () => {
    if (!hasSelectedGenre) return undefined;
    if (itemsLoading) return undefined;
    if (items.length === 0) return `No ${noun.plural} in this genre`;
    return `Showing ${items.length} ${noun.plural}, page ${genresPage} of ${totalPages}`;
  };

  const handleClearGenre = () => {
    pendingRestoreGenreIdRef.current = genreId ?? null;
    onClearGenre();
  };

  const handlePageChange = (newPage: number) => {
    onPageChange(newPage);
    scrollWindowToTop();
  };

  if (genresLoading) {
    return <LibraryGenresTabSkeleton />;
  }

  if (genresError || isApiFailure(genresRes)) {
    return (
      <MoviesLoadError
        message={
          isApiFailure(genresRes)
            ? genresRes.message
            : "Couldn’t load genres. Check your connection and try again."
        }
        onRetry={() => void refetchGenres()}
      />
    );
  }

  if (genres.length === 0) {
    return (
      <LibraryEmptyState
        icon={emptyIcon}
        message={`No genres with ${noun.plural} in your library yet.`}
      />
    );
  }

  return (
    <div>
      <ul
        className={
          hasSelectedGenre
            ? "mb-5 grid grid-cols-3 gap-2 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-7"
            : "mb-5 grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6"
        }
        aria-label={genresListLabel}
      >
        {genres.map(g => {
          const selected = genreId === g.genre_id;
          return (
            <li key={g.genre_id} className="min-w-0">
              <button
                type="button"
                ref={node => {
                  if (genreButtonRefs.current === null) {
                    genreButtonRefs.current = new Map();
                  }
                  if (node) {
                    genreButtonRefs.current.set(g.genre_id, node);
                    return;
                  }
                  genreButtonRefs.current.delete(g.genre_id);
                }}
                onClick={() => onSelectGenre(g.genre_id)}
                className={cn(
                  "flex w-full min-w-0 flex-col justify-between rounded-lg border text-left",
                  MOTION_MICRO_CONTROL_CLASS,
                  FOCUS_VISIBLE_RING_CLASS,
                  hasSelectedGenre ? "min-h-14 p-2" : "min-h-20 p-3",
                  selected
                    ? "border-primary bg-primary text-primary-foreground shadow-lg shadow-primary/15"
                    : "border-border bg-muted/70 text-foreground hover:border-primary/40 hover:bg-muted",
                )}
                aria-pressed={selected}
              >
                <span className="line-clamp-2 text-sm font-semibold">
                  {g.genre_tag}
                </span>
                <span
                  className={`${hasSelectedGenre ? "mt-1" : "mt-3"} text-xs ${
                    selected ? "text-primary-foreground/70" : "text-muted-foreground"
                  }`}
                >
                  {g.count} {g.count === 1 ? noun.singular : noun.plural}
                </span>
              </button>
            </li>
          );
        })}
      </ul>

      {hasSelectedGenre && (
        <>
          <LiveAnnouncer message={getAnnouncement()} />

          <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <span className="text-sm font-medium text-foreground">
                {selectedGenreTag ?? "Genre"}
              </span>
              <span className="text-sm text-muted-foreground">
                {total.toLocaleString()} {noun.plural}
              </span>
              <button
                type="button"
                onClick={handleClearGenre}
                className={cn(
                  "inline-flex shrink-0 items-center gap-1 rounded-full border border-border px-3 py-1 text-xs font-medium text-muted-foreground hover:border-border hover:bg-muted hover:text-foreground",
                  MOTION_MICRO_CONTROL_CLASS,
                  FOCUS_VISIBLE_RING_CLASS,
                )}
                aria-label="Clear genre filter"
              >
                <X className="size-3.5" aria-hidden="true" />
                Clear
              </button>
            </div>
            <div className="flex flex-wrap items-center gap-2 sm:gap-3">
              {hasMultiplePages && (
                <span className="text-sm text-muted-foreground">
                  Page {genresPage} of {totalPages}
                </span>
              )}
              <LibrarySortToggle sort={sort} onToggle={onSortToggle} />
            </div>
          </div>

          {itemsError || isApiFailure(itemsRes) ? (
            <MoviesLoadError
              message={
                isApiFailure(itemsRes)
                  ? itemsRes.message
                  : `Couldn’t load ${noun.plural} for this genre. Check your connection and try again.`
              }
              onRetry={() => void refetchItems()}
            />
          ) : itemsLoading ? (
            <LibraryAllTabSkeleton perPage={perPage} />
          ) : items.length === 0 ? (
            <LibraryEmptyState
              icon={emptyIcon}
              message={`No ${noun.plural} found for this genre.`}
            />
          ) : (
            <>
              <div className={`mb-8 ${LIBRARY_POSTER_GRID_CLASS}`}>
                {items.map(item => (
                  <Fragment key={item.id}>{renderCard(item)}</Fragment>
                ))}
              </div>
              {totalPages > 1 && (
                <LibraryPagination
                  currentPage={genresPage}
                  totalPages={totalPages}
                  onPageChange={handlePageChange}
                />
              )}
            </>
          )}
        </>
      )}
    </div>
  );
}

// Mirrors the unselected genre-chip grid above: ten chip-sized boxes.
export function LibraryGenresTabSkeleton() {
  return (
    <div>
      <div className="mb-5 grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
        {Array.from({ length: 10 }).map((_, i) => (
          <div
            key={i}
            className={cn(
              "min-h-20 rounded-lg border border-border bg-card",
              MOTION_LOADING_STATE_CLASS,
            )}
          />
        ))}
      </div>
    </div>
  );
}
