import {
  Fragment,
  useEffect,
  useRef,
  type ReactNode,
  type Ref,
  type RefObject,
} from "react";
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
import LibrarySortToggle from "@/components/shared/LibrarySortToggle";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import SkeletonStatus from "@/components/shared/SkeletonStatus";
import LoadErrorAlert from "@/components/shared/LoadErrorAlert";
import { Button } from "@/components/ui/button";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import { usePosterFallback } from "@/hooks/usePosterFallback";
import {
  CARD_MEDIA_HOVER_CLASS,
  FOCUS_VISIBLE_RING_CLASS,
  LIBRARY_POSTER_GRID_CLASS,
  MEDIA_FALLBACK_SURFACE_CLASS,
  MOTION_LOADING_STATE_CLASS,
  MOTION_MICRO_CONTROL_CLASS,
  TMDB_TILE_BACKDROP_SIZE,
} from "@/lib/constants";
import { nounForCount } from "@/lib/format";
import { apiErrorMessage, isApiFailure } from "@/lib/is-api-failure";
import { scrollWindowToTop } from "@/lib/motion";
import { buildTmdbImageUrl } from "@/lib/tmdb-image-url";
import { cn } from "@/lib/utils";
import type { SortOrder } from "@/types";


// One tile geometry whether or not a genre is selected: the grid used to swap
// from tall cards to compact chips on selection and re-flow every tile.
// Columns follow the room left beside the sidebar, which appears at md: a tile
// stays wide enough to show its backdrop. Phones get the shorter tile, so a
// selected genre's results are not pushed a screen away.
const GENRE_TILE_GRID_CLASS =
  "mb-5 grid grid-cols-3 gap-2 lg:grid-cols-5 xl:grid-cols-7";
const GENRE_TILE_HEIGHT_CLASS = "h-20 sm:h-24";
/**
 * One genre tile: the page maps its API row (movie_count, show_count, …) onto
 * `count`, and passes the row's backdrop path (null when the genre has none).
 */
export type LibraryGenre = {
  genre_id: number;
  genre_tag: string;
  count: number;
  backdrop_path: string | null;
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
  sort: SortOrder;
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

// The genre facet of a library page: a grid of genre tiles, and once one is
// pressed, a paginated grid of its items beneath. Clearing the filter returns
// focus to the tile that was selected, so keyboard users are not dropped at
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
    // The error alert speaks for itself (role="alert"); an empty list here
    // would announce "none in this genre" over it.
    if (itemsError || isApiFailure(itemsRes)) return undefined;
    if (items.length === 0) return `No ${noun.plural} in this genre`;
    return `Showing ${items.length} ${nounForCount(items.length, noun)}, page ${genresPage} of ${totalPages}`;
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
      <LoadErrorAlert
        message={apiErrorMessage(genresRes, "Couldn’t load genres.")}
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
      <ul className={GENRE_TILE_GRID_CLASS} aria-label={genresListLabel}>
        {genres.map(g => {
          return (
            <li key={g.genre_id} className="min-w-0">
              <GenreTile
                genre={g}
                noun={noun}
                selected={genreId === g.genre_id}
                onSelect={() => onSelectGenre(g.genre_id)}
                buttonRef={node => {
                  if (genreButtonRefs.current === null) {
                    genreButtonRefs.current = new Map();
                  }
                  if (node) {
                    genreButtonRefs.current.set(g.genre_id, node);
                    return;
                  }
                  genreButtonRefs.current.delete(g.genre_id);
                }}
              />
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
                {total.toLocaleString()} {nounForCount(total, noun)}
              </span>
              <Button
                variant="outline"
                size="sm"
                shape="pill"
                onClick={handleClearGenre}
                aria-label="Clear genre filter"
              >
                <X aria-hidden="true" />
                Clear
              </Button>
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
            <LoadErrorAlert
              message={apiErrorMessage(itemsRes, `Couldn’t load ${noun.plural} for this genre.`)}
              onRetry={() => void refetchItems()}
            />
          ) : itemsLoading ? (
            <SkeletonStatus label={`Loading ${noun.plural}`}>
              <LibraryAllTabSkeleton perPage={perPage} />
            </SkeletonStatus>
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

type GenreTileProps = {
  genre: LibraryGenre;
  noun: LibraryNoun;
  selected: boolean;
  onSelect: () => void;
  buttonRef: Ref<HTMLButtonElement>;
};

// A media tile, not a Button look: a backdrop from the genre with the name and
// count over a band at the bottom (design-system §3.2). The band is opaque
// behind the text and fades out above it, so the text keeps its contrast on any
// image; pressed, the band turns primary and the image stays above it. A raw
// <button> because the whole tile is the control.
function GenreTile({ genre, noun, selected, onSelect, buttonRef }: GenreTileProps) {
  const backdropUrl = buildTmdbImageUrl(genre.backdrop_path, TMDB_TILE_BACKDROP_SIZE);
  const { showPoster: showBackdrop, onError } = usePosterFallback(backdropUrl);

  return (
    <button
      type="button"
      ref={buttonRef}
      onClick={onSelect}
      className={cn(
        "group relative block w-full min-w-0 overflow-hidden rounded-lg border text-left",
        GENRE_TILE_HEIGHT_CLASS,
        MEDIA_FALLBACK_SURFACE_CLASS,
        MOTION_MICRO_CONTROL_CLASS,
        FOCUS_VISIBLE_RING_CLASS,
        selected
          ? "border-primary shadow-lg shadow-primary/15"
          : "border-border hover:border-primary/40",
      )}
      aria-pressed={selected}
    >
      {showBackdrop && (
        <img
          src={backdropUrl}
          alt=""
          width={500}
          height={281}
          loading="lazy"
          decoding="async"
          fetchPriority="low"
          className={cn("absolute inset-0 size-full object-cover", CARD_MEDIA_HOVER_CLASS)}
          onError={onError}
        />
      )}
      <span className="absolute inset-x-0 bottom-0 flex flex-col">
        <span
          aria-hidden="true"
          className={cn(
            "h-5 bg-linear-to-t to-transparent",
            selected ? "from-primary" : "from-black/70",
          )}
        />
        <span
          className={cn(
            "flex flex-col px-2 pb-2",
            selected ? "bg-primary" : "bg-black/70",
          )}
        >
          <span
            className={cn(
              "line-clamp-2 text-sm/tight font-semibold",
              selected ? "text-primary-foreground" : "text-white",
            )}
          >
            {genre.genre_tag}
          </span>
          <span
            className={cn(
              "mt-0.5 text-xs",
              // /85 keeps the 12 px count above AA on the primary band in
              // both themes; /70 measured 3.74:1 in light.
              selected ? "text-primary-foreground/85" : "text-white/80",
            )}
          >
            {genre.count} {nounForCount(genre.count, noun)}
          </span>
        </span>
      </span>
    </button>
  );
}

// Mirrors the genre-tile grid above: ten tile-sized boxes.
export function LibraryGenresTabSkeleton() {
  return (
    <SkeletonStatus label="Loading genres">
      <div className={GENRE_TILE_GRID_CLASS}>
        {Array.from({ length: 10 }).map((_, i) => (
          <div
            key={i}
            className={cn(
              "rounded-lg border border-border bg-card",
              GENRE_TILE_HEIGHT_CLASS,
              MOTION_LOADING_STATE_CLASS,
            )}
          />
        ))}
      </div>
    </SkeletonStatus>
  );
}
