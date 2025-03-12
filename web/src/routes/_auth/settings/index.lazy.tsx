import { createLazyFileRoute, Link } from "@tanstack/solid-router";
import { FiUsers } from "solid-icons/fi";

export const Route = createLazyFileRoute("/_auth/settings/")({
  component: SettingsPage,
});

function SettingsPage() {
  return (
    <main class='container mx-auto px-4 py-8'>
      <div class='max-w-6xl mx-auto'>
        <header class='mb-8'>
          <h1 class='text-3xl font-bold text-white'>Settings</h1>
          <p class='mt-2 text-sky-200'>
            Manage your application settings and preferences
          </p>
        </header>

        <nav class='border-b border-sky-200/10'>
          <ul class='flex gap-2'>
            <li>
              <Link
                to='/settings/users'
                search={{ page: 1, limit: 10 }}
                activeProps={{ class: "text-sky-400 border-sky-400" }}
                inactiveProps={{
                  class:
                    "text-sky-200 border-transparent hover:text-sky-300 hover:border-sky-300",
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
