import { useQuery, type QueryKey } from "@tanstack/react-query";
import type { LucideIcon } from "lucide-react";
import {
  type LibraryNoun,
  type LibraryQueryOptions,
} from "@/components/shared/LibraryAllTab";
import { MoviesLoadError } from "@/components/shared/MoviesLoadError";
import { nounForCount } from "@/lib/format";
import { isApiFailure } from "@/lib/is-api-failure";

/** One count in the stats line: its icon, visible label and how to read it. */
export type LibraryStatsFigure<TData> = {
  icon: LucideIcon;
  /** The visible label beside the count, e.g. "Movies". */
  label: string;
  /** Lowercase nouns for the region's accessible name. */
  noun: LibraryNoun;
  getValue: (data: TData) => number;
};

type LibraryStatsProps<
  TData extends Record<string, unknown>,
  TKey extends QueryKey,
> = {
  queryOpts: LibraryQueryOptions<TData, TKey>;
  figures: LibraryStatsFigure<TData>[];
};

// The count line beside a library page's header. The whole figure is one
// labelled region, so a screen reader hears "Library statistics: 42 movies"
// (or "…: 3 albums, 40 tracks, 2 musicians") once instead of an icon, a
// number and a word as separate stops per figure.
export default function LibraryStats<
  TData extends Record<string, unknown>,
  TKey extends QueryKey,
>({ queryOpts, figures }: LibraryStatsProps<TData, TKey>) {
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

  const payload = data?.error === false ? data.data : null;
  const counts = figures.map(figure =>
    payload ? figure.getValue(payload) : 0,
  );
  const regionLabel = isLoading
    ? "Library statistics: loading"
    : `Library statistics: ${figures
        .map((figure, i) => `${counts[i]} ${nounForCount(counts[i], figure.noun)}`)
        .join(", ")}`;

  return (
    <section className="flex flex-wrap gap-x-6 gap-y-3" aria-label={regionLabel}>
      {figures.map(({ icon: Icon, label }, i) => (
        <div key={label} className="flex items-center gap-2" aria-hidden="true">
          <Icon className="size-4 text-primary" />
          <span className="font-medium text-foreground">
            {isLoading ? "—" : counts[i]}
          </span>
          <span className="text-muted-foreground">{label}</span>
        </div>
      ))}
    </section>
  );
}
