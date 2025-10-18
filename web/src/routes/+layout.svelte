<script lang="ts">
  import "../app.css";
  import "$lib/icons";
  import favicon from "$lib/assets/favicon.svg";
  import { fade } from "svelte/transition";
  import Header from "$lib/components/Header.svelte";

  let { children } = $props();
</script>

<svelte:head>
  <link rel="icon" href={favicon} />
</svelte:head>

<a
  href="#main"
  class="sr-only focus:not-sr-only focus:fixed focus:top-4 focus:left-4 focus:z-50
	       focus:px-4 focus:py-2 focus:rounded-xl focus:bg-yellow-400 focus:text-slate-900"
>
  Skip to content
</a>

<div
  class="
		flex min-h-dvh flex-col
		bg-gradient-to-b from-slate-900 via-blue-950/80 to-slate-900
	"
>
  <Header />

  <main
    id="main"
    class="flex-1 mx-auto w-full max-w-7xl px-4 sm:px-6 lg:px-8 py-6"
    in:fade={{ duration: 150 }}
    out:fade={{ duration: 100 }}
  >
    {@render children?.()}
  </main>

  <footer
    class="border-t border-slate-800 bg-slate-900/80 text-sm text-slate-500 py-4"
  >
    <div
      class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 flex justify-between items-center"
    >
      <p>&copy; {new Date().getFullYear()} Igloo Media Center</p>
      <p class="italic text-slate-600">Built with 🧊 and SvelteKit</p>
    </div>
  </footer>
</div>
