import { createResource, Switch, Match, For } from "solid-js";
import Spinner from "../shared/Spinner";
import MovieCard from "../shared/MovieCard";
import Alert from "../shared/Alert";
import { getLatestMovies } from "../http/movieRequest";
import type { Component } from "solid-js";

const LatestMovies: Component = () => {
  const [movies] = createResource(getLatestMovies);

  return (
    <>
      <h2 class='text-2xl font-bold mb-4'>Latest Movies</h2>

      <Switch>
        <Match when={movies.loading}>
          <Spinner />
        </Match>
        <Match when={movies.error}>
          <Alert
            title='Error'
            msg='Unable to fetch movies'
            time={6000}
            variant='danger'
          />
        </Match>
        <Match when={movies()}>
          <div class='grid grid-cols-2 sm:grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4'>
            <For each={movies().data}>{m => <MovieCard movie={m} />}</For>
          </div>
        </Match>
      </Switch>
    </>
  );
};

export default LatestMovies;
