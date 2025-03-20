import { createLazyFileRoute, Link } from "@tanstack/solid-router";
import { FiUsers, FiSettings } from "solid-icons/fi";

export const Route = createLazyFileRoute("/_auth/settings/")({
  component: SettingsPage,
});

function SettingsPage() {
  return (
    <main class='container mx-auto px-4 py-8'>
      <div class='max-w-6xl mx-auto'>
        <header class='mb-8'>
          <div class="flex items-center gap-3 mb-2">
            <FiSettings class="w-8 h-8 text-yellow-300" aria-hidden="true" />
            <h1 class='text-3xl font-bold'>
              <span class='bg-gradient-to-r from-yellow-300 to-yellow-400 bg-clip-text text-transparent'>
                Settings
              </span>
            </h1>
          </div>

          <p class='mt-2 text-blue-200'>
            Manage your application settings and preferences
          </p>
        </header>

        <nav class='border-b border-blue-800/20'>
          <ul class='flex gap-2'>
            <li>
              <Link
                to='/settings/users'
                from="/settings"
                search={{ page: 1, limit: 10 }}
                activeProps={{ class: "text-yellow-300 border-yellow-300" }}
                inactiveProps={{
                  class:
                    "text-blue-200 border-transparent hover:text-yellow-300 hover:border-yellow-300",
                }}
                class='inline-flex items-center gap-2 px-4 py-3 text-sm font-medium border-b-2 transition-colors'
              >
                <FiUsers class='w-4 h-4' aria-hidden='true' />
                Manage Users
              </Link>
            </li>
          </ul>
        </nav>
      </div>
    </main>
  );
}
