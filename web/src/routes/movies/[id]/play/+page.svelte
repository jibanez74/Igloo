<script>
  export let data;
</script>

<h1>Video Player</h1>

<div class="container mx-auto">
  <video width="100%" height="100%" autoplay controls muted>
    <source src={data.filePath} type="video/mp4" />
    Your browser does not support the video tag.
  </video>
</div>
