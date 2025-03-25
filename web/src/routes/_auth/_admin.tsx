import { Outlet, createFileRoute, Link } from "@tanstack/solid-router";
import { authState } from "../../stores/authStore";
import { FiUsers, FiSettings, FiBarChart2 } from "solid-icons/fi";
export const Route = createFileRoute("/_auth/_admin")({
  beforeLoad: () => {
    if (!authState.user?.is_admin) {
      // throw new Error("403 - You do not have permission to access this page");
      console.log("user is not an admin");
    }
  },
  component: AdminLayout,
});

function AdminLayout() {
  return (
    <div class="min-h-screen bg-blue-950">
      <header class="bg-blue-950/50 backdrop-blur-sm border-b border-blue-900/20">
        <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div class="flex items-center justify-between h-16">
            <h1 class="text-xl font-bold text-yellow-300">Admin Dashboard</h1>
          </div>

          <nav class="flex -mb-px" aria-label="Admin navigation">
            <div class="border-b border-blue-800 w-full">
              <div class="flex space-x-8">
                <Link
                  to="/settings"
                  from={Route.fullPath}
                  class="inline-flex items-center px-4 py-2 text-sm font-medium text-white hover:text-yellow-300 border-b-2 border-transparent hover:border-yellow-300 transition-colors relative group"
                >
                  <FiSettings
                    class="w-4 h-4 mr-2 text-yellow-300"
                    aria-hidden="true"
                  />
                  Settings
                  <span class="absolute inset-x-0 -bottom-[2px] h-[2px] bg-gradient-to-r from-transparent via-yellow-300/50 to-transparent opacity-0 group-hover:opacity-100 transition-opacity" />
                </Link>

                <Link
                  to="/users"
                  from={Route.fullPath}
                  class="inline-flex items-center px-4 py-2 text-sm font-medium text-white hover:text-yellow-300 border-b-2 border-transparent hover:border-yellow-300 transition-colors relative group"
                >
                  <FiUsers
                    class="w-4 h-4 mr-2 text-yellow-300"
                    aria-hidden="true"
                  />
                  Users
                  <span class="absolute inset-x-0 -bottom-[2px] h-[2px] bg-gradient-to-r from-transparent via-yellow-300/50 to-transparent opacity-0 group-hover:opacity-100 transition-opacity" />
                </Link>

                <Link
                  to="/stats"
                  from={Route.fullPath}
                  class="inline-flex items-center px-4 py-2 text-sm font-medium text-white hover:text-yellow-300 border-b-2 border-transparent hover:border-yellow-300 transition-colors relative group"
                >
                  <FiBarChart2
                    class="w-4 h-4 mr-2 text-yellow-300"
                    aria-hidden="true"
                  />
                  Stats
                  <span class="absolute inset-x-0 -bottom-[2px] h-[2px] bg-gradient-to-r from-transparent via-yellow-300/50 to-transparent opacity-0 group-hover:opacity-100 transition-opacity" />
                </Link>
              </div>
            </div>
          </nav>
        </div>
      </header>

      <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <Outlet />
      </main>
    </div>
  );
}
