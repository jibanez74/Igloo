import { Switch, Match, For, Show } from "solid-js";
import { createQuery } from "@tanstack/solid-query";
import MovieCard from "./MovieCard";
import ErrorWarning from "./ErrorWarning";
import Spinner from "./Spinner";
import type { SimpleMovie } from "../types/Movie";

type LatestMoviesResponse = {
  movies: SimpleMovie[];
};

export default function LatestMovies() {
  const query = createQuery(() => ({
    queryKey: ["latest-movies"],
    queryFn: async (): Promise<LatestMoviesResponse> => {
      try {
        const res = await fetch("/api/v1/movies/latest", {
          credentials: "include",
        });

        const data = await res.json();

        if (!res.ok) {
          throw new Error(
            `${res.status} - ${data.error ? data.error : res.statusText}`
          );
        }

        return data;
      } catch (err) {
        console.error(err);
        throw new Error(
          "a network error occurred while fetching latest movies"
        );
      }
    },
  }));

  return (
    <section class="py-8">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div class="flex items-center justify-between h-10 mb-6">
          <h2 class="text-2xl font-bold text-white">
            <span class="bg-gradient-to-r from-yellow-300 to-yellow-400 bg-clip-text text-transparent">
              Latest Movies
            </span>
          </h2>

          <div class="w-5 h-5 flex items-center justify-center">
            <Show when={query.isPending}>
              <Spinner />
            </Show>
          </div>
        </div>

        <div class="h-10">
          <ErrorWarning
            error={query.error?.message || ""}
            isVisible={query.isError}
          />
        </div>

        <div class="min-h-[200px]">
          <Show
            when={!query.isPending}
            fallback={
              <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4 sm:gap-6">
                <For each={Array(12).fill(null)}>
                  {() => (
                    <div class="aspect-[2/3] rounded-xl bg-blue-950/50 animate-pulse" />
                  )}
                </For>
              </div>
            }
          >
            <Switch>
              <Match when={query.data?.movies.length === 0}>
                <div class="h-full flex items-center justify-center">
                  <p class="text-blue-200/80">No movies available</p>
                </div>
              </Match>
              <Match when={query.data?.movies}>
                <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-4 sm:gap-6">
                  <For each={query.data?.movies}>
                    {(movie) => <MovieCard movie={movie} />}
                  </For>
                </div>
              </Match>
            </Switch>
          </Show>
        </div>
      </div>
    </section>
  );
}
