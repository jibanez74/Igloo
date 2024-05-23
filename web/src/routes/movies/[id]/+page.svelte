<script>
  export let data;
</script>

<video controls autoplay muted height="100%" width="100%">
  <source src={data.url} type="video/mp4" />
  Your browser does not support the video tag.
</video>
