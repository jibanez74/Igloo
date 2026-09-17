import { useQuery, type QueryKey } from "@tanstack/react-query";
import type { LucideIcon } from "lucide-react";
import {
  type LibraryNoun,
  type LibraryQueryOptions,
} from "@/components/shared/LibraryAllTab";
import { MoviesLoadError } from "@/components/shared/MoviesLoadError";
import { isApiFailure } from "@/lib/is-api-failure";

type LibraryStatsProps<
  TData extends Record<string, unknown>,
  TKey extends QueryKey,
> = {
  queryOpts: LibraryQueryOptions<TData, TKey>;
  getTotal: (data: TData) => number;
  icon: LucideIcon;
  /** The visible label beside the count, e.g. "Movies". */
  label: string;
  /** Lowercase nouns for the region's accessible name. */
  noun: LibraryNoun;
};

// The count line beside a library page's header. The whole figure is one
// labelled region, so a screen reader hears "Library statistics: 42 movies"
// once instead of an icon, a number and a word as three stops.
export default function LibraryStats<
  TData extends Record<string, unknown>,
  TKey extends QueryKey,
>({
  queryOpts,
  getTotal,
  icon: Icon,
  label,
  noun,
}: LibraryStatsProps<TData, TKey>) {
  const { data, isError, isLoading, refetch } = useQuery(queryOpts);

  if (isError || isApiFailure(data)) {
    return (
      <MoviesLoadError
        message={
          isApiFailure(data)
            ? data.message
            : "Couldn’t load library statistics. Check your connection and try again."
        }
        onRetry={() => void refetch()}
      />
    );
  }

  const total = data?.error === false ? getTotal(data.data) : 0;
  const regionLabel = isLoading
    ? "Library statistics: loading"
    : `Library statistics: ${total} ${noun.plural}`;

  return (
    <section className="flex flex-wrap gap-6" aria-label={regionLabel}>
      <div className="flex items-center gap-2" aria-hidden="true">
        <Icon className="size-4 text-primary" />
        <span className="font-medium text-foreground">
          {isLoading ? "—" : total}
        </span>
        <span className="text-muted-foreground">{label}</span>
      </div>
    </section>
  );
}
