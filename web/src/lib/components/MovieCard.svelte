<script>
  export let id;
  export let thumb;
  export let title;
  export let year;
</script>

<div class="movie-card">
  <a href={`/movies/${id}`}>
    <img src={thumb} alt={title} />
  </a>

  <div class="movie-card-info">
    <h3>
      {title}
    </h3>

    <p>
      {year}
    </p>
  </div>
</div>
