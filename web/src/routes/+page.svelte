<script>
  import MovieCard from '$lib/components/MovieCard.svelte'

export let data;
</script>

<h1>Welcome to Igloo</h1>

<div>
  {#each data.movies as movie(movie.ID)}
  <MovieCard id={movie.ID} title={movie.Title} thumb={movie.Thumb} year={movie.Year} />
  {/each}
</div>