<script>
  export let data;

  let directPlay = false;
</script>

<h1>
  {data.Title}
</h1>

<a href={`/movies/${data.ID}/play?direct_play=${directPlay}&file_path=${data.FilePath}`}>
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
