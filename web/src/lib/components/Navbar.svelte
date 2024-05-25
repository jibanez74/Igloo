<script>
  import NavLink from './NavLink.svelte';
</script>

<nav class="bg-blue-700 border-b border-blue-500">
  <div class="mx-auto max-w-7xl px-2 sm:px-6 lg:px-8">
    <div class="relative flex h-20 items-center justify-between">
      <div class="absolute inset-y-0 left-0 flex items-center md:hidden">
        <!-- Mobile menu button-->
        <button
          type="button"
          id="mobile-dropdown-button"
          class="relative inline-flex items-center justify-center rounded-md p-2 text-gray-400 hover:bg-gray-700 hover:text-white focus:outline-none focus:ring-2 focus:ring-inset focus:ring-white"
          aria-controls="mobile-menu"
          aria-expanded="false"
        >
          <span class="absolute -inset-0.5"></span>
          <span class="sr-only">Open main menu</span>
          <svg
            class="block h-6 w-6"
            fill="none"
            viewBox="0 0 24 24"
            stroke-width="1.5"
            stroke="currentColor"
            aria-hidden="true"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5"
            />
          </svg>
        </button>
      </div>

      <div class="flex flex-1 items-center justify-center md:items-stretch md:justify-start">
        <!-- Logo -->
        <a class="flex flex-shrink-0 items-center" href="/index.html">
          <enhanced:img class="h-10 w-auto" src="/static/images/igloo-icon.png" alt="logo" />

          <span class="hidden md:block text-white text-2xl font-bold ml-2">Igloo</span>
        </a>
        <!-- Desktop Menu Hidden below md screens -->
        <div class="hidden md:ml-6 md:block">
          <div class="flex space-x-2">
            <NavLink href="/" text="Home" />
            <NavLink href="/movies" text="Movies" />
            <NavLink href="/tv-shows" text="TV Shows" />
            <NavLink href="/music" text="Music" />
          </div>
        </div>
      </div>

      <!-- Right Side Menu (Logged Out) -->
      <div class="hidden md:block md:ml-6">
        <div class="flex items-center">
          <button
            class="flex items-center text-white bg-gray-700 hover:bg-gray-900 hover:text-white rounded-md px-3 py-2"
          >
            <i class="fa-brands fa-google text-white mr-2"></i>
            <span>Login or Register</span>
          </button>
        </div>
      </div>

      <!-- Right Side Menu (Logged In) -->
      <div
        class="absolute inset-y-0 right-0 flex items-center pr-2 md:static md:inset-auto md:ml-6 md:pr-0"
      >
        <!-- Profile dropdown button -->
        <div class="relative ml-3">
          <div>
            <button
              type="button"
              class="relative flex rounded-full bg-gray-800 text-sm focus:outline-none focus:ring-2 focus:ring-white focus:ring-offset-2 focus:ring-offset-gray-800"
              id="user-menu-button"
              aria-expanded="false"
              aria-haspopup="true"
            >
              <span class="absolute -inset-1.5"></span>
              <span class="sr-only">Open user menu</span>
              <enhanced:img
                class="h-8 w-8 rounded-full"
                src="/static/images/profile.png"
                alt="dashboard"
              />
            </button>
          </div>

          <!-- Profile dropdown -->
          <div
            id="user-menu"
            class="hidden absolute right-0 z-10 mt-2 w-48 origin-top-right rounded-md bg-white py-1 shadow-lg ring-1 ring-black ring-opacity-5 focus:outline-none"
            role="menu"
            aria-orientation="vertical"
            aria-labelledby="user-menu-button"
            tabindex="-1"
          >
            <a
              href="/dashboard"
              class="block px-4 py-2 text-sm text-gray-700"
              role="menuitem"
              tabindex="-1"
              id="user-menu-item-0">Your Profile</a
            >
            <a
              href="/logout"
              class="block px-4 py-2 text-sm text-gray-700"
              role="menuitem"
              tabindex="-1"
              id="user-menu-item-2">Sign Out</a
            >
          </div>
        </div>
      </div>
    </div>
  </div>

  <!-- Mobile menu, show/hide based on menu state. -->
  <div class="hidden" id="mobile-menu">
    <div class="space-y-1 px-2 pb-3 pt-2">
      <a href="/" class="bg-gray-900 text-white block rounded-md px-3 py-2 text-base font-medium"
        >Home</a
      >

      <button
        class="flex items-center text-white bg-gray-700 hover:bg-gray-900 hover:text-white rounded-md px-3 py-2 my-4"
      >
        <i class="fa-brands fa-google mr-2"></i>
        <span>Login or Register</span>
      </button>
    </div>
  </div>
</nav>
