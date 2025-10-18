<script>
  import { FontAwesomeIcon } from '@fortawesome/svelte-fontawesome';
  import { faEnvelope, faLock } from '@fortawesome/free-solid-svg-icons';
</script>

<!-- full-page wrapper -->
<div class="min-h-dvh flex flex-col items-center justify-center bg-gradient-to-b from-slate-900 via-blue-950/80 to-slate-900 px-4">
  <!-- Card -->
  <div class="w-full max-w-md rounded-2xl border border-slate-800 bg-slate-900/70 backdrop-blur p-8 shadow-xl">
    <!-- Header -->
    <div class="text-center mb-8">
      <h1 class="text-3xl font-bold text-sky-400">Welcome to Igloo</h1>
      <p class="mt-2 text-slate-400">Log in to your account to access your media library.</p>
    </div>

    <form class="space-y-6">
      <div>
        <label for="email" class="block text-sm font-medium text-slate-300 mb-1">Email</label>
        <div class="relative">
          <span class="absolute inset-y-0 left-0 flex items-center pl-3 text-slate-400">
            <FontAwesomeIcon icon={faEnvelope} class="w-5 h-5" />
          </span>
          <input
          autofocus
          type="email"
            id="email"
            name="email"
            required
            placeholder="you@example.com"
            class="w-full rounded-xl bg-slate-800/70 border border-slate-700 text-slate-100 placeholder-slate-500
                   pl-10 pr-3 py-2 focus:outline-none focus:ring-2 focus:ring-yellow-400 focus:border-yellow-400"
          />
        </div>
      </div>

      <div>
        <label for="password" class="block text-sm font-medium text-slate-300 mb-1">Password</label>
        <div class="relative">
          <span class="absolute inset-y-0 left-0 flex items-center pl-3 text-slate-400">
            <FontAwesomeIcon icon={faLock} class="w-5 h-5" />
          </span>
          <input
            id="password"
            type="password"
            bind:value={password}
            minlength="8"
            required
            placeholder="••••••••"
            class="w-full rounded-xl bg-slate-800/70 border border-slate-700 text-slate-100 placeholder-slate-500
                   pl-10 pr-3 py-2 focus:outline-none focus:ring-2 focus:ring-yellow-400 focus:border-yellow-400"
          />
        </div>
      </div>

      <!-- Submit -->
      <div class="pt-2">
        <button
          type="submit"
          class="w-full rounded-xl bg-yellow-400 hover:bg-yellow-300 text-slate-900 font-semibold py-2
                 transition focus:outline-none focus:ring-2 focus:ring-yellow-400"
        >
          Log In
        </button>
      </div>
    </form>

    <!-- Footer -->
    <p class="mt-6 text-center text-sm text-slate-500">
      Don’t have an account yet?
      <a href="/register" class="text-sky-400 hover:text-yellow-400 font-medium focus:outline-none focus:ring-2 focus:ring-yellow-400 rounded">
        Sign up
      </a>
    </p>
  </div>
</div>