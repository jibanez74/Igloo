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
import {
  LIBRARY_POSTER_GRID_CLASS,
  MOTION_LOADING_STATE_CLASS,
} from "@/lib/constants";
import { nounForCount } from "@/lib/format";
import { isApiFailure } from "@/lib/is-api-failure";
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

/** The pagination fields every paginated library payload carries. */
export type LibraryPageData = {
  total: number;
  page: number;
  per_page: number;
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
  /** Cards per page; sizes the loading skeleton to the grid it replaces. */
  perPage: number;
  noun: LibraryNoun;
  emptyIcon: LucideIcon;
  onPageChange: (page: number) => void;
  /** The grid the cards sit in; defaults to the 2:3 poster grid. */
  gridClassName?: string;
  /** One placeholder card, repeated per page; defaults to the poster card's. */
  skeletonCard?: ReactNode;
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
  onPageChange,
  onSortToggle,
  gridClassName = LIBRARY_POSTER_GRID_CLASS,
  skeletonCard,
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

  if (isLoading) {
    return (
      <LibraryAllTabSkeleton
        perPage={perPage}
        gridClassName={gridClassName}
        skeletonCard={skeletonCard}
        withSortToggle={hasSort}
      />
    );
  }

  if (isError || isApiFailure(data)) {
    return (
      <MoviesLoadError
        message={
          isApiFailure(data)
            ? data.message
            : `Couldn’t load ${noun.plural}. Check your connection and try again.`
        }
        onRetry={() => void refetch()}
      />
    );
  }

  if (items.length === 0) {
    return (
      <>
        <LiveAnnouncer message={getAnnouncement()} />
        <LibraryEmptyState
          icon={emptyIcon}
          message={`No ${noun.plural} found in your library.`}
        />
      </>
    );
  }

  return (
    <div>
      <LiveAnnouncer message={getAnnouncement()} />

      {/* Header with page info and sort toggle */}
      {(hasMultiplePages || hasSort) && (
        <div
          className={
            hasMultiplePages && hasSort
              ? "mb-5 flex items-center justify-between gap-2"
              : "mb-5 flex justify-end"
          }
        >
          {hasMultiplePages && (
            <span className="text-sm text-muted-foreground">
              Page {currentPage} of {totalPages}
            </span>
          )}
          {hasSort && (
            <div className="flex items-center gap-2 sm:gap-3">
              <LibrarySortToggle sort={sort} onToggle={onSortToggle} />
            </div>
          )}
        </div>
      )}

      <div className={cn("mb-8", gridClassName)}>
        {items.map(item => (
          <Fragment key={item.id}>{renderCard(item)}</Fragment>
        ))}
      </div>

      {totalPages > 1 && (
        <LibraryPagination
          currentPage={currentPage}
          totalPages={totalPages}
          onPageChange={handlePageChange}
        />
      )}
    </div>
  );
}

type LibraryAllTabSkeletonProps = {
  perPage: number;
  gridClassName?: string;
  skeletonCard?: ReactNode;
  /** Mirrors whether the real tab has a sort toggle to reserve room for. */
  withSortToggle?: boolean;
};

// Authored beside the grid it mirrors (design-system §3.4): the same columns
// and the same card boxes, one per card on a full page.
export function LibraryAllTabSkeleton({
  perPage,
  gridClassName = LIBRARY_POSTER_GRID_CLASS,
  skeletonCard = <PosterCardSkeleton />,
  withSortToggle = true,
}: LibraryAllTabSkeletonProps) {
  return (
    <div>
      {withSortToggle && (
        <div className="mb-5 flex justify-end">
          <div className={cn("h-8 w-16 rounded-full bg-muted", MOTION_LOADING_STATE_CLASS)} />
        </div>
      )}
      <div className={cn("mb-8", gridClassName)}>
        {Array.from({ length: perPage }).map((_, i) => (
          <Fragment key={i}>{skeletonCard}</Fragment>
        ))}
      </div>
    </div>
  );
}
