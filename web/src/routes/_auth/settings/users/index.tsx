import { createFileRoute } from "@tanstack/solid-router";
import UsersTable from "../../../../components/UsersTable";
import Pagination from "../../../../components/Pagination";
// import { FiUserPlus } from "solid-icons/fi";
import type { PaginationSearch } from "../../../../types/Pagination";
import type { PaginationResponse } from "../../../../types/Pagination";
import type { SimpleUser } from "../../../../types/User";

export const Route = createFileRoute("/_auth/settings/users/")({
  component: UsersPage,
  validateSearch: (search: Record<string, unknown>): PaginationSearch => ({
    page: Number(search?.page ?? 1),
    limit: Number(search?.limit ?? 10),
  }),
  loaderDeps: ({ search }) => search,
  loader: async ({ deps }): Promise<PaginationResponse<SimpleUser>> => {
    try {
      const res = await fetch(
        `/api/v1/users?page=${deps.page}&limit=${deps.limit}`,
        {
          credentials: "include",
        }
      );

      const data = await res.json();

      if (!res.ok) {
        throw new Error(
          `${res.status} - ${data.error ? data.error : res.statusText}`
        );
      }

      return data;
    } catch (err) {
      console.error(err);
      throw new Error("Failed to fetch users");
    }
  },
});

function UsersPage() {
  const data = Route.useLoaderData();
  const { total_pages, count, items } = data();
  const search = Route.useSearch();
  const { page, limit } = search();
  const navigate = Route.useNavigate();

  const handlePageChange = (newPage: number) => {
    navigate({
      search: {
        page: newPage,
        limit,
      },
    });
  };

  return (
    <main class="container mx-auto px-4 py-8">
      <section
        class="min-h-[400px] bg-slate-800/50 backdrop-blur-sm rounded-2xl p-6 shadow-lg"
        aria-label="Users list"
      >
        <header class="mb-6 flex items-center justify-between">
          <div class="flex items-center gap-4">
            <h2 class="text-2xl font-bold text-white">Users</h2>
            {/* <Link
              to="/settings/users/create"
              search={{ page: 1, limit: 10 }}
              class="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-sky-500/10 rounded-lg hover:bg-sky-500/20 transition-colors"
            >
              <FiUserPlus class="w-4 h-4" aria-hidden="true" />
              New User
            </Link> */}
          </div>
          <p class="text-sm text-sky-200">
            Total users:{" "}
            <span class="text-white font-medium">{count}</span>
          </p>
        </header>

        {items.length === 0 ? (
          <p
            class="h-40 flex items-center justify-center text-sky-200"
            role="status"
          >
            No users found
          </p>
        ) : (
          <>
            <div class="overflow-x-auto mb-6">
              <UsersTable users={items} />
            </div>
            <Pagination
              currentPage={page}
              totalPages={total_pages}
              onPageChange={handlePageChange}
            />
          </>
        )}
      </section>
    </main>
  );
}
