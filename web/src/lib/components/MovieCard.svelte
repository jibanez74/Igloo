<script>
  export let id;
  export let title;
  export let thumb;
  export let year;
</script>

<div class="max-w-sm mx-auto bg-primary hover:bg-secondary shadow-md rounded-lg overflow-hidden">
  <a href={`/movies/${id}`}>
    <img src={thumb} alt={title} class="w-full object-cover" />
  </a>

  <div class="p-5 text-xl text-center">
    <h5 class="mb-2 font-bold">{title}</h5>
    <p class="mb-2">{year}</p>
  </div>
</div>
