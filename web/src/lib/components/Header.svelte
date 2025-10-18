<script lang="ts">
  import { page } from "$app/stores";
  import { derived } from "svelte/store";
  import { FontAwesomeIcon } from "@fortawesome/svelte-fontawesome";
  import {
    faHouse,
    faFilm,
    faTv,
    faMusic,
    faGear,
    faRightFromBracket,
  } from "@fortawesome/free-solid-svg-icons";

  const nav = [
    { name: "Home", href: "/", icon: faHouse },
    { name: "Movies", href: "/movies", icon: faFilm },
    { name: "TV Shows", href: "/shows", icon: faTv },
    { name: "Music", href: "/music", icon: faMusic },
    { name: "Settings", href: "/settings", icon: faGear },
  ];

  const active = derived(page, $page => $page.url.pathname || "/");
  $: currentPath = $active;

  let mobileOpen = false;
  const closeMobile = () => (mobileOpen = false);
</script>

<header
  class="sticky top-0 z-50 border-b border-slate-800 bg-slate-900/80 backdrop-blur supports-[backdrop-filter]:bg-slate-900/60"
>
  <div
    class="mx-auto max-w-7xl h-16 px-4 sm:px-6 lg:px-8 flex items-center gap-4"
  >
    <!-- Brand -->
    <a
      href="/"
      class="flex items-center gap-2 rounded-lg focus:outline-none focus:ring-2 focus:ring-yellow-400"
    >
      <div
        class="size-8 grid place-items-center rounded-lg bg-sky-500/15 text-sky-400"
      >
        🧊
      </div>
      <span class="text-lg font-semibold tracking-tight">Igloo</span>
    </a>

    <!-- Desktop nav -->
    <nav class="ml-2 hidden md:flex items-center gap-1" aria-label="Primary">
      {#each nav as item}
        {@const isActive =
          currentPath === item.href ||
          (currentPath !== "/" && currentPath.startsWith(item.href))}
        <a
          href={item.href}
          class="flex items-center gap-2 rounded-lg px-3 py-2 text-sm font-medium transition
                 hover:text-yellow-400 focus:outline-none focus:ring-2 focus:ring-yellow-400
                 {isActive
            ? 'text-sky-300 bg-slate-800/60'
            : 'text-slate-200/90'}"
          aria-current={isActive ? "page" : undefined}
        >
          <FontAwesomeIcon icon={item.icon} class="w-4 h-4" />
          <span>{item.name}</span>
        </a>
      {/each}
    </nav>

    <!-- Spacer -->
    <div class="ms-auto"></div>

    <!-- Sign Out (swap to your auth action) -->
    <form method="POST" action="/logout" class="hidden md:block">
      <button
        type="submit"
        class="flex items-center gap-2 rounded-xl border border-slate-700 bg-slate-800/60 px-3 py-2 text-sm font-medium
               hover:bg-slate-700/60 hover:text-yellow-400
               focus:outline-none focus:ring-2 focus:ring-yellow-400"
      >
        <FontAwesomeIcon icon={faRightFromBracket} class="w-4 h-4" />
        <span>Sign Out</span>
      </button>
    </form>

    <!-- Mobile menu button -->
    <button
      class="md:hidden rounded-lg p-2 text-slate-200/90 hover:text-yellow-400 hover:bg-slate-800/60
             focus:outline-none focus:ring-2 focus:ring-yellow-400"
      aria-label="Toggle menu"
      aria-controls="mobile-nav"
      aria-expanded={mobileOpen}
      on:click={() => (mobileOpen = !mobileOpen)}
    >
      <!-- hamburger / close -->
      {#if !mobileOpen}
        <svg
          class="size-6"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <path d="M4 6h16M4 12h16M4 18h16" />
        </svg>
      {:else}
        <svg
          class="size-6"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
        >
          <path d="M6 18L18 6M6 6l12 12" />
        </svg>
      {/if}
    </button>
  </div>

  <!-- Mobile nav panel -->
  <div
    id="mobile-nav"
    class="md:hidden border-t border-slate-800 bg-slate-900/95"
    hidden={!mobileOpen}
  >
    <nav
      class="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-3 space-y-1"
      aria-label="Primary mobile"
    >
      {#each nav as item}
        {@const isActive =
          currentPath === item.href ||
          (currentPath !== "/" && currentPath.startsWith(item.href))}
        <a
          href={item.href}
          class="flex items-center gap-3 rounded-lg px-3 py-2 text-base font-medium
                 hover:text-yellow-400 hover:bg-slate-800/60
                 focus:outline-none focus:ring-2 focus:ring-yellow-400
                 {isActive
            ? 'text-sky-300 bg-slate-800/60'
            : 'text-slate-200/90'}"
          aria-current={isActive ? "page" : undefined}
          on:click={closeMobile}
        >
          <FontAwesomeIcon icon={item.icon} class="w-4 h-4" />
          <span>{item.name}</span>
        </a>
      {/each}

      <!-- Sign Out -->
      <form method="POST" action="/logout" class="pt-2">
        <button
          type="submit"
          class="flex items-center justify-center gap-3 w-full rounded-lg border border-slate-700 bg-slate-800/60 px-3 py-2 text-base font-medium
                 hover:bg-slate-700/60 hover:text-yellow-400
                 focus:outline-none focus:ring-2 focus:ring-yellow-400"
          on:click={closeMobile}
        >
          <FontAwesomeIcon icon={faRightFromBracket} class="w-4 h-4" />
          <span>Sign Out</span>
        </button>
      </form>
    </nav>
  </div>
</header>
