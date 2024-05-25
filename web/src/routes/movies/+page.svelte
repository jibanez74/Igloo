<script>
  import MovieCard from '$lib/components/MovieCard.svelte';

  export let data;
</script>

<h1>Movie Gallery</h1>

<div class="container mx-auto p-5">
  <div class="grid gap-5 grid-cols-2 md:grid-cols-4 lg:grid-cols-3">
    {#each data.movies as movie (movie.ID)}
      <MovieCard id={movie.ID} title={movie.Title} thumb={movie.Thumb} year={movie.Year} />
    {/each}
  </div>
</div>
