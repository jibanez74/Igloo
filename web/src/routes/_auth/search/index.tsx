import { createFileRoute } from "@tanstack/react-router";
import { z } from "zod";
import {
  searchAlbumsQueryOpts,
  searchAllQueryOpts,
  searchMoviesQueryOpts,
  searchMusiciansQueryOpts,
  searchTracksQueryOpts,
} from "@/lib/query-opts";
import { SEARCH_PER_PAGE } from "@/lib/constants";

const searchSchema = z.object({
  q: z.string().catch("").default(""),
  tab: z
    .enum(["all", "movies", "albums", "musicians", "tracks"])
    .catch("all")
    .default("all"),
  page: z.number().int().positive().catch(1).default(1),
});

export type SearchParams = z.infer<typeof searchSchema>;

export const Route = createFileRoute("/_auth/search/")({
  validateSearch: searchSchema,
  loaderDeps: ({ search: { q, tab, page } }) => ({ q, tab, page }),
  loader: async ({ context, deps: { q, tab, page } }) => {
    const trimmed = q.trim();
    if (!trimmed) return;

    const { queryClient } = context;
    if (tab === "all") {
      await queryClient.ensureQueryData(searchAllQueryOpts(trimmed));
      return;
    }

    if (tab === "movies") {
      await queryClient.ensureQueryData(
        searchMoviesQueryOpts(trimmed, page, SEARCH_PER_PAGE),
      );

      return;
    }

    if (tab === "albums") {
      await queryClient.ensureQueryData(
        searchAlbumsQueryOpts(trimmed, page, SEARCH_PER_PAGE),
      );

      return;
    }

    if (tab === "musicians") {
      await queryClient.ensureQueryData(
        searchMusiciansQueryOpts(trimmed, page, SEARCH_PER_PAGE),
      );

      return;
    }

    if (tab === "tracks") {
      await queryClient.ensureQueryData(
        searchTracksQueryOpts(trimmed, page, SEARCH_PER_PAGE),
      );
    }
  },
});
