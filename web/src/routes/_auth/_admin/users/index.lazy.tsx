import { Show } from "solid-js";
import { Link, createLazyFileRoute } from "@tanstack/solid-router";
import { createQuery } from "@tanstack/solid-query";
import UsersTable from "../../../../components/UsersTable";
import Spinner from "../../../../components/Spinner";
import ErrorWarning from "../../../../components/ErrorWarning";
import { FiUserPlus } from "solid-icons/fi";
import type { SimpleUser } from "../../../../types/User";

type UsersResponse = {
  items: SimpleUser[];
  count: number;
};

export const Route = createLazyFileRoute("/_auth/_admin/users/")({
  component: UsersPage,
});

function UsersPage() {
  const query = createQuery(() => ({
    queryKey: ["users"],
    queryFn: async (): Promise<UsersResponse> => {
      try {
        const res = await fetch("/api/v1/users", {
          credentials: "same-origin",
        });

        const data = await res.json();

        if (!res.ok) {
          throw new Error(
            `${res.status} - ${data.error ? data.error : res.statusText}`
          );
        }

        return data;
      } catch (err) {
        console.error(err);
        throw new Error("a network error occurred while fetching users");
      }
    },
  }));

  return (
    <div class="container mx-auto">
      <section
        class="bg-blue-950/50 backdrop-blur-sm rounded-2xl p-6 shadow-lg shadow-blue-900/20 border border-blue-900/20"
        aria-label="Users list"
      >
        <header class="mb-6 flex items-center justify-between">
          <div class="flex items-center gap-4">
            <h1 class="text-2xl font-bold text-yellow-300">Users</h1>
            <Link
              to="/users/form"
              search={{
                update: false,
              }}
              class="inline-flex items-center gap-2 px-4 py-2 text-sm font-medium text-blue-950 bg-yellow-300 rounded-lg hover:bg-yellow-400 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:ring-offset-2 focus:ring-offset-blue-950 transition-colors shadow-sm shadow-black/20"
            >
              <FiUserPlus class="w-4 h-4" aria-hidden="true" />
              New User
            </Link>
          </div>

          <Show when={query.isSuccess}>
            <p class="text-sm text-white">
              Total users:{" "}
              <span class="text-yellow-300 font-medium">
                {query.data?.count ?? 0}
              </span>
            </p>
          </Show>
        </header>

        <Show
          when={!query.isLoading}
          fallback={
            <div class="h-40 flex items-center justify-center">
              <Spinner size="lg" />
            </div>
          }
        >
          <Show
            when={!query.isError}
            fallback={
              <div class="h-40 flex items-center justify-center">
                <ErrorWarning
                  error={query.error?.message ?? ""}
                  isVisible={true}
                />
              </div>
            }
          >
            <Show
              when={query.data?.items.length}
              fallback={
                <p
                  class="h-40 flex items-center justify-center text-white"
                  role="status"
                >
                  No users found
                </p>
              }
            >
              <div class="overflow-x-auto">
                <UsersTable users={query.data!.items} />
              </div>
            </Show>
          </Show>
        </Show>
      </section>
    </div>
  );
}
