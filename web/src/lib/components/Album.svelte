<script lang="ts">
  import { FontAwesomeIcon } from "@fortawesome/svelte-fontawesome";
  import { faPlay } from "@fortawesome/free-solid-svg-icons";
  import type { SimpleAlbum } from "$lib/types";

  let { id, title, cover, musician_name }: SimpleAlbum = $props();
</script>

<a
  href={`/music/albums/${id}`}
  class="group relative block rounded-2xl overflow-hidden border border-slate-800
         bg-slate-800/40 hover:bg-slate-800/60 focus:outline-none focus:ring-2
         focus:ring-yellow-400 transition-all duration-300"
  aria-label={`${title} by ${musician_name}`}
>
  <div class="relative aspect-square bg-slate-700 overflow-hidden">
    <img
      src={cover}
      alt={`${title} cover`}
      class="h-full w-full object-cover transition-transform duration-300
             group-hover:scale-105 group-hover:brightness-90"
      loading="lazy"
      decoding="async"
    />

    <!-- hover overlay -->
    <div
      class="absolute inset-0 flex items-center justify-center
             bg-slate-900/60 opacity-0 group-hover:opacity-100
             transition-opacity duration-300"
    >
      <!-- play button -->
      <button
        type="button"
        class="rounded-full bg-yellow-400 text-slate-900 p-4 shadow-lg
               hover:scale-110 active:scale-95 transition-transform
               focus:outline-none focus:ring-2 focus:ring-yellow-400"
        aria-label={`play ${title}`}
      >
        <FontAwesomeIcon icon={faPlay} class="w-5 h-5" />
      </button>
    </div>
  </div>

  <div class="p-3">
    <h3 class="line-clamp-1 font-semibold text-slate-100">{title}</h3>
    <p class="mt-0.5 line-clamp-1 text-sm text-slate-400">{musician_name}</p>
  </div>
</a>
