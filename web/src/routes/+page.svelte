<script>
  import MovieCard from '$lib/components/MovieCard.svelte';

  export let data;
</script>

<div class="container mx-auto p-4">
  <div class="grid gap-5 grid-cols-2 md:grid-cols-4 lg:grid-cols-3">
    {#each data.movies as movie (movie.ID)}
      <MovieCard title={movie.Title} thumb={movie.Thumb} year={movie.Year} id={movie.ID} />
    {/each}
  </div>
</div>
