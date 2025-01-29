import { getRouteApi, createFileRoute, Link } from "@tanstack/react-router";
import UsersTable from "@/components/UsersTable";
import Pagination from "@/components/Pagination";
import { FiUserPlus } from "react-icons/fi";
import type { UsersResponse } from "@/types/User";
import type { PaginationSearch } from "@/types/Pagination";

export const Route = createFileRoute("/settings/users/")({
  component: UsersPage,
  validateSearch: (search: Record<string, unknown>): PaginationSearch => ({
    page: Number(search?.page ?? 1),
    limit: Number(search?.limit ?? 10),
  }),
  loaderDeps: ({ search }) => search,
  loader: async ({ deps }): Promise<UsersResponse> => {
    const res = await fetch(
      `/api/v1/users?page=${deps.page}&limit=${deps.limit}`
    );

    if (!res.ok) {
      throw new Error(`${res.status} - ${res.statusText}`);
    }

    return await res.json();
  },
});

function UsersPage() {
  const { useLoaderData, useSearch } = getRouteApi("/settings/users/");
  const { users, count, pages } = useLoaderData();
  const { page, limit } = useSearch();

  return (
    <main className='container mx-auto px-4 py-8'>
      <section
        className='min-h-[400px] bg-slate-800/50 backdrop-blur-sm rounded-2xl p-6 shadow-lg'
        aria-label='Users list'
      >
        <header className='mb-6 flex items-center justify-between'>
          <div className='flex items-center gap-4'>
            <h2 className='text-2xl font-bold text-white'>Users</h2>
            <Link
              to='/settings/users/create'
              search={{ page: 1, limit: 10 }}
              className='inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-white bg-sky-500/10 rounded-lg hover:bg-sky-500/20 transition-colors'
            >
              <FiUserPlus className='w-4 h-4' aria-hidden='true' />
              New User
            </Link>
          </div>
          <p className='text-sm text-sky-200'>
            Total users: <span className='text-white font-medium'>{count}</span>
          </p>
        </header>

        {users.length === 0 ? (
          <p
            className='h-40 flex items-center justify-center text-sky-200'
            role='status'
          >
            No users found
          </p>
        ) : (
          <>
            <div className='overflow-x-auto mb-6'>
              <UsersTable users={users} />
            </div>
            <Pagination
              currentPage={page}
              totalPages={pages}
              baseUrl='/settings/users'
              searchParams={{ limit: limit.toString() }}
            />
          </>
        )}
      </section>
    </main>
  );
}
