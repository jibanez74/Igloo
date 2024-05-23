<script>
  import MovieCard from '$lib/components/MovieCard.svelte';

export let data;
</script>

<h1>Movie Gallery</h1>

<p>
  You have {data.count} movies, you are on page {data.page} and there a total of {data.pages} pages
</p>

{#each data.movies as movie (movie.ID)}
  <MovieCard id={movie.ID} title={movie.Title} thumb={movie.Thumb} year={movie.Year} />
{/each}
