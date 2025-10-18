<script lang="ts">
  import Album from "$lib/components/Album.svelte";
  import type { SimpleAlbum } from "$lib/types";

  let {
    albums = [],
    title = "Albums",
    subtitle = "Albums Collection",
  }: { albums: SimpleAlbum[]; title: string; subtitle: string } = $props();

  const noAlbumsMessage = "There are currently no albums to show.";
</script>

<section aria-labelledby="albums-heading" class="mt-8">
  <div
    class="mb-6 flex flex-col sm:flex-row sm:items-end sm:justify-between gap-3"
  >
    <div>
      <h2
        id="albums-heading"
        class="text-2xl font-semibold text-slate-100 tracking-tight"
      >
        {title}
      </h2>

      <p class="text-sm text-slate-400">{subtitle}</p>
    </div>

    <div class="flex items-center gap-2">
      <button
        class="rounded-xl border border-slate-700 bg-slate-800/50 px-3 py-1.5 text-sm text-slate-300
				       hover:text-yellow-400 hover:bg-slate-700/60
				       focus:outline-none focus:ring-2 focus:ring-yellow-400"
      >
        Sort
      </button>

      <button
        class="rounded-xl border border-slate-700 bg-slate-800/50 px-3 py-1.5 text-sm text-slate-300
				       hover:text-yellow-400 hover:bg-slate-700/60
				       focus:outline-none focus:ring-2 focus:ring-yellow-400"
      >
        Filter
      </button>
    </div>
  </div>

  {#if albums.length > 0}
    <ul
      role="list"
      class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-6 2xl:grid-cols-8 gap-4"
    >
      {#each albums as album}
        <li>
          <Album
            id={album.id}
            title={album.title}
            cover={album.cover}
            musician_name={album.musician_name}
          />
        </li>
      {/each}
    </ul>
  {:else}
    <div
      class="rounded-2xl border border-slate-800 bg-slate-800/40 p-8 text-center"
    >
      <p class="text-slate-400">{noAlbumsMessage}</p>
    </div>
  {/if}
</section>
