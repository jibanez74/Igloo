import { createFileRoute, redirect } from "@tanstack/react-router";
import {
  searchAlbumsQueryOpts,
  searchAllQueryOpts,
  searchMoviesQueryOpts,
  searchMusiciansQueryOpts,
  searchTracksQueryOpts,
} from "@/lib/query-opts";
import { SEARCH_PER_PAGE } from "@/lib/constants";
import { searchSearchSchema } from "@/types/route-search";
import type { SearchTab } from "@/types";

type PagedSearchTab = Exclude<SearchTab, "all">;

function redirectToLastSearchPage({
  q,
  tab,
  requestedPage,
  totalPages,
}: {
  q: string;
  tab: PagedSearchTab;
  requestedPage: number;
  totalPages: number;
}) {
  if (totalPages === 0 || requestedPage <= totalPages) {
    return;
  }

  throw redirect({
    to: "/search",
    search: {
      q,
      tab,
      page: totalPages,
    },
    replace: true,
  });
}

export const Route = createFileRoute("/_auth/search/")({
  validateSearch: searchSearchSchema,
  loaderDeps: ({ search: { q, tab, page } }) => ({ q, tab, page }),
  loader: async ({ context, deps: { q, tab, page } }) => {
    const trimmed = q.trim();
    if (!trimmed) return;

    const { queryClient } = context;
    if (tab === "all") {
      await queryClient.ensureQueryData(searchAllQueryOpts(trimmed));
      return;
    }

    const result =
      tab === "movies"
        ? await queryClient.ensureQueryData(
            searchMoviesQueryOpts(trimmed, page, SEARCH_PER_PAGE),
          )
        : tab === "albums"
          ? await queryClient.ensureQueryData(
              searchAlbumsQueryOpts(trimmed, page, SEARCH_PER_PAGE),
            )
          : tab === "musicians"
            ? await queryClient.ensureQueryData(
                searchMusiciansQueryOpts(trimmed, page, SEARCH_PER_PAGE),
              )
            : await queryClient.ensureQueryData(
                searchTracksQueryOpts(trimmed, page, SEARCH_PER_PAGE),
              );

    if (result.error === false) {
      redirectToLastSearchPage({
        q: trimmed,
        tab,
        requestedPage: page,
        totalPages: result.data.total_pages,
      });
    }
  },
});
