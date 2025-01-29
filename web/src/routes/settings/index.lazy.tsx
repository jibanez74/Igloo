import { createLazyFileRoute, Link } from "@tanstack/react-router";
import { FiUsers } from "react-icons/fi";

export const Route = createLazyFileRoute("/settings/")({
  component: SettingsPage,
});

function SettingsPage() {
  return (
    <main className='container mx-auto px-4 py-8'>
      <div className='max-w-6xl mx-auto'>
        <header className='mb-8'>
          <h1 className='text-3xl font-bold text-white'>Settings</h1>
          <p className='mt-2 text-sky-200'>
            Manage your application settings and preferences
          </p>
        </header>

        <nav className='border-b border-sky-200/10'>
          <ul className='flex gap-2'>
            <li>
              <Link
                to='/settings/users'
                search={{ page: 1, limit: 10 }}
                activeProps={{ className: "text-sky-400 border-sky-400" }}
                inactiveProps={{
                  className:
                    "text-sky-200 border-transparent hover:text-sky-300 hover:border-sky-300",
                }}
                className='inline-flex items-center gap-2 px-4 py-3 text-sm font-medium border-b-2 transition-colors'
              >
                <FiUsers className='w-4 h-4' aria-hidden='true' />
                Manage Users
              </Link>
            </li>
          </ul>
        </nav>
      </div>
    </main>
  );
}
