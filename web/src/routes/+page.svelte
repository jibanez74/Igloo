<script lang="ts">
  import AlbumsGrid from "$lib/components/AlbumsGrid.svelte";
  import type { PageProps } from "./$types";

  let { data }: PageProps = $props();
</script>

<h1>Hello from Igloo</h1>

<section>
  <div>
    <h2>Latest Movies Go here</h2>
  </div>
</section>

<AlbumsGrid
  albums={data.albums}
  title="Latest Albums"
  subtitle="Recently added albums"
/>
