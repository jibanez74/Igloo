<script>
  export let data;
</script>

<h1>
  {data.Title}
</h1>

<a href={`/movies/${data.ID}/play`}>
Play Movie
</a>

<p>
  Release Date: {data.ReleaseDate}
</p>

<p>
  {data.TagLine}
</p>

<p>
  {data.Summary}
</p>