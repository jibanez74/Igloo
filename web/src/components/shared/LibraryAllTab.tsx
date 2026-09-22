import { Fragment, useEffect, type ReactNode } from "react";
import {
  useQuery,
  type QueryKey,
  type UseQueryOptions,
} from "@tanstack/react-query";
import type { LucideIcon } from "lucide-react";
import LibraryEmptyState from "@/components/shared/LibraryEmptyState";
import LibraryPagination from "@/components/shared/LibraryPagination";
import LibrarySortToggle, {
  type LibrarySortDirection,
} from "@/components/shared/LibrarySortToggle";
import LiveAnnouncer from "@/components/shared/LiveAnnouncer";
import { MoviesLoadError } from "@/components/shared/MoviesLoadError";
import { PosterCardSkeleton } from "@/components/shared/PosterCard";
import { LIBRARY_POSTER_GRID_CLASS } from "@/lib/constants";
import { nounForCount } from "@/lib/format";
import { apiErrorMessage, isApiFailure } from "@/lib/is-api-failure";
import { scrollWindowToTop } from "@/lib/motion";
import { cn } from "@/lib/utils";
import type { ApiResponseType } from "@/types";

/**
 * The query a library tab owns: the page prepares the queryOptions(), the tab
 * runs it, so the loader and the tab always agree on the key.
 */
export type LibraryQueryOptions<
  TData extends Record<string, unknown>,
  TKey extends QueryKey = QueryKey,
> = UseQueryOptions<ApiResponseType<TData>, Error, ApiResponseType<TData>, TKey>;

/**
 * The pagination fields a library tab actually reads. `page` and `per_page`
 * also ride along in every payload, but no tab reads them back — the caller
 * already knows both, so requiring them here bought nothing.
 */
export type LibraryPageData = {
  total: number;
  total_pages: number;
};

/** Lowercase nouns for copy and announcements: { singular: "movie", plural: "movies" }. */
export type LibraryNoun = {
  singular: string;
  plural: string;
};

/**
 * The sort direction travels with its toggle handler: a list whose API sorts
 * (movies, shows) passes both, a list whose API does not (albums, musicians)
 * passes neither and renders no toggle.
 */
type LibrarySortProps =
  | {
      sort: LibrarySortDirection;
      onSortToggle: () => void;
    }
  | {
      sort?: undefined;
      onSortToggle?: undefined;
    };

type LibraryAllTabProps<
  TItem extends { id: number },
  TData extends LibraryPageData & Record<string, unknown>,
  TKey extends QueryKey,
> = {
  queryOpts: LibraryQueryOptions<TData, TKey>;
  getItems: (data: TData) => TItem[];
  renderCard: (item: TItem) => ReactNode;
  currentPage: number;
  /**
   * Cards per page; sizes the loading skeleton to the grid it replaces. Not
   * redundant with the payload: it is the only page size available *before* the
   * query resolves, which is exactly when the skeleton needs it.
   */
  perPage: number;
  noun: LibraryNoun;
  emptyIcon: LucideIcon;
  /**
   * The empty sentence; defaults to "No {plural} found in your library.",
   * which a list scoped to something narrower than the library overrides.
   */
  emptyMessage?: string;
  onPageChange: (page: number) => void;
  /** The grid the cards sit in; defaults to the 2:3 poster grid. */
  gridClassName?: string;
  /** One placeholder card, repeated per page; defaults to the poster card's. */
  skeletonCard?: ReactNode;
  /**
   * The left side of the toolbar, opposite the page info and sort toggle: the
   * liked-movies view puts its "Back to playlists" button and count here.
   */
  toolbarStartSlot?: ReactNode;
} & LibrarySortProps;

// The "everything" tab of a library page: one paginated poster grid, sortable
// when the API sorts. Navigation stays with the page (it owns the typed route
// search), so the tab only reports what the user asked for.
export default function LibraryAllTab<
  TItem extends { id: number },
  TData extends LibraryPageData & Record<string, unknown>,
  TKey extends QueryKey,
>({
  queryOpts,
  getItems,
  renderCard,
  currentPage,
  sort,
  perPage,
  noun,
  emptyIcon,
  emptyMessage = `No ${noun.plural} found in your library.`,
  onPageChange,
  onSortToggle,
  gridClassName = LIBRARY_POSTER_GRID_CLASS,
  skeletonCard,
  toolbarStartSlot,
}: LibraryAllTabProps<TItem, TData, TKey>) {
  const { data, isLoading, isError, refetch } = useQuery(queryOpts);

  const items = data?.error === false ? getItems(data.data) : [];
  const totalPages = data?.error === false ? data.data.total_pages : 0;
  const hasMultiplePages = totalPages > 1;
  const hasSort = sort !== undefined;

  // The API does not clamp the page, so an out-of-range page (a hand-edited URL,
  // or a scan that shrank the library underneath us) answers with no items but a
  // positive total. The empty state below has no pagination to escape from, so
  // walk the reader back to the last real page instead of stranding them.
  useEffect(() => {
    if (totalPages > 0 && currentPage > totalPages) {
      onPageChange(totalPages);
    }
  }, [currentPage, totalPages, onPageChange]);

  const getAnnouncement = () => {
    if (isLoading) return undefined;
    if (items.length === 0) return `No ${noun.plural} found`;
    return `Showing ${items.length} ${nounForCount(items.length, noun)}, page ${currentPage} of ${totalPages}`;
  };

  const handlePageChange = (newPage: number) => {
    onPageChange(newPage);
    scrollWindowToTop();
  };

  // One toolbar, rendered identically in every state. The skeleton cannot know
  // `totalPages` before the query resolves, so a toolbar that appeared only once
  // the data landed moved the grid down under it (design-system §3.4) — which is
  // why the row is reserved here rather than mirrored in the skeleton.
  const toolbar = (
    <div
      data-slot="library-tab-toolbar"
      className="mb-5 flex min-h-8 flex-wrap items-center justify-between gap-3"
    >
      <div className="flex flex-wrap items-center gap-3">{toolbarStartSlot}</div>
      <div className="flex flex-wrap items-center gap-2 sm:gap-3">
        {hasMultiplePages && (
          <span className="text-sm text-muted-foreground">
            Page {currentPage} of {totalPages}
          </span>
        )}
        {/* A URL toggle works while the page is in flight, so it renders during
            loading too rather than as a placeholder pill. */}
        {hasSort && <LibrarySortToggle sort={sort} onToggle={onSortToggle} />}
      </div>
    </div>
  );

  const renderBody = () => {
    if (isLoading) {
      return (
        <LibraryAllTabSkeleton
          perPage={perPage}
          gridClassName={gridClassName}
          skeletonCard={skeletonCard}
        />
      );
    }

    if (isError || isApiFailure(data)) {
      return (
        <MoviesLoadError
          message={apiErrorMessage(data, `Couldn’t load ${noun.plural}. Check your connection and try again.`)}
          onRetry={() => void refetch()}
        />
      );
    }

    if (items.length === 0) {
      return (
        <LibraryEmptyState icon={emptyIcon} message={emptyMessage} />
      );
    }

    return (
      <>
        <div className={cn("mb-8", gridClassName)}>
          {items.map(item => (
            <Fragment key={item.id}>{renderCard(item)}</Fragment>
          ))}
        </div>

        {hasMultiplePages && (
          <LibraryPagination
            currentPage={currentPage}
            totalPages={totalPages}
            onPageChange={handlePageChange}
          />
        )}
      </>
    );
  };

  return (
    <div>
      <LiveAnnouncer message={getAnnouncement()} />
      {toolbar}
      {renderBody()}
    </div>
  );
}

type LibraryAllTabSkeletonProps = {
  perPage: number;
  gridClassName?: string;
  skeletonCard?: ReactNode;
};

// Authored beside the grid it mirrors (design-system §3.4): the same columns
// and the same card boxes, one per card on a full page. The toolbar above the
// grid belongs to the tab, which reserves it in every state.
export function LibraryAllTabSkeleton({
  perPage,
  gridClassName = LIBRARY_POSTER_GRID_CLASS,
  skeletonCard = <PosterCardSkeleton />,
}: LibraryAllTabSkeletonProps) {
  return (
    <div className={cn("mb-8", gridClassName)}>
      {Array.from({ length: perPage }).map((_, i) => (
        <Fragment key={i}>{skeletonCard}</Fragment>
      ))}
    </div>
  );
}
