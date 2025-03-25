import { createSignal } from "solid-js";
import { createFileRoute } from "@tanstack/solid-router";
import { createMutation } from "@tanstack/solid-query";
import { FiUser, FiMail, FiLock, FiShield, FiX, FiSave } from "solid-icons/fi";
import type { UserForm } from "../../../../types/User";

type FormParams = {
  id?: number;
  update: boolean;
};

type UserResponse = {
  user: UserForm;
  update: boolean;
};

export const Route = createFileRoute("/_auth/_admin/users/form")({
  validateSearch: (search: Record<string, unknown>): FormParams => ({
    id: Number(search?.id ?? 0),
    update: Boolean(search?.update),
  }),
  loaderDeps: ({ search }) => search,
  loader: async ({ deps }): Promise<UserResponse> => {
    const result = {
      user: {
        id: deps.id ?? 0,
        name: "",
        email: "",
        username: "",
        password: "",
        is_admin: false,
        is_active: false,
      },
      update: deps.update,
    };

    if (deps.id) {
      try {
        const res = await fetch(`/api/v1/users/${deps.id}`, {
          credentials: "same-origin",
        });

        const data = await res.json();

        if (!res.ok) {
          throw new Error(
            `${res.status} - ${data.error ? data.error : res.statusText}`
          );
        }

        result.update = true;
        result.user = data.user;
      } catch (err) {
        console.error(err);
        throw new Error(
          `a network error occurred while fetching user with id of ${deps.id}`
        );
      }
    }

    return result;
  },
  component: UserFormPage,
});

function UserFormPage() {
  const navigate = Route.useNavigate();
  const data = Route.useLoaderData();
  const { user, update } = data();
  const ctx = Route.useRouteContext();
  const { queryClient } = ctx();

  console.log(update)

  const [name, setName] = createSignal(user.name);
  const [username, setUsername] = createSignal(user.username);
  const [email, setEmail] = createSignal(user.email);
  const [password, setPassword] = createSignal(user.password);
  const [isAdmin, setIsAdmin] = createSignal(false);
  const [isActive, setIsActive] = createSignal(true);

  const mutation = createMutation(() => ({
    mutationFn: async () => {
      try {
        const input = {
          name: name(),
          email: email(),
          username: username(),
          is_admin: isAdmin(),
          is_active: isActive,
        };

        const res = await fetch(`/api/v1/users/update/${user?.id}`, {
          method: "put",
          credentials: "same-origin",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify(input),
        });

        if (!res.ok) {
          const data = await res.json();

          throw new Error(
            `${res.status} - ${data.error ? data.error : res.statusText}`
          );
        }

        queryClient.invalidateQueries({
          queryKey: ["users"],
        });
      } catch (err) {
        console.error(err);
        throw new Error("a network error occurred while submitting the form");
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["users"] });
      navigate({
        to: "/users",
        from: Route.fullPath,
      });
    },
  }));

  const handleSubmit = (e: Event) => {
    e.preventDefault();
    mutation.mutate();
  };

  return (
    <div class="container mx-auto">
      <section
        class="bg-blue-950/50 backdrop-blur-sm rounded-2xl p-6 shadow-lg shadow-blue-900/20 border border-blue-900/20"
        aria-labelledby="user-form-title"
      >
        <header class="mb-8">
          <h1 id="user-form-title" class="text-2xl font-bold text-yellow-300">
            Create New User
          </h1>
          <p class="mt-2 text-blue-200">
            Fill in the information below to create a new user account.
          </p>
        </header>

        <form class="space-y-6" onSubmit={handleSubmit}>
          {/* Name Field */}
          <div>
            <label for="name" class="block text-sm font-medium text-white mb-2">
              Full Name
            </label>
            <div class="relative">
              <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                <FiUser class="h-5 w-5 text-yellow-300" aria-hidden="true" />
              </div>
              <input
                autofocus
                type="text"
                id="name"
                name="name"
                required
                minLength={2}
                maxLength={60}
                value={name()}
                disabled={mutation.isPending}
                onInput={(e) => setName(e.currentTarget.value)}
                class="block w-full pl-10 pr-3 py-2 bg-blue-900/50 border border-blue-800 rounded-lg text-white placeholder-blue-300/50 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed"
              />
            </div>
          </div>

          {/* Username Field */}
          <div>
            <label
              for="username"
              class="block text-sm font-medium text-white mb-2"
            >
              Username
            </label>
            <div class="relative">
              <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                <FiUser class="h-5 w-5 text-yellow-300" aria-hidden="true" />
              </div>
              <input
                type="text"
                id="username"
                name="username"
                required
                minLength={2}
                maxLength={20}
                value={username()}
                disabled={mutation.isPending}
                onInput={(e) => setUsername(e.currentTarget.value)}
                class="block w-full pl-10 pr-3 py-2 bg-blue-900/50 border border-blue-800 rounded-lg text-white placeholder-blue-300/50 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed"
              />
            </div>
          </div>

          {/* Email Field */}
          <div>
            <label
              for="email"
              class="block text-sm font-medium text-white mb-2"
            >
              Email Address
            </label>
            <div class="relative">
              <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                <FiMail class="h-5 w-5 text-yellow-300" aria-hidden="true" />
              </div>
              <input
                type="email"
                id="email"
                name="email"
                required
                value={email()}
                disabled={mutation.isPending}
                onInput={(e) => setEmail(e.currentTarget.value)}
                class="block w-full pl-10 pr-3 py-2 bg-blue-900/50 border border-blue-800 rounded-lg text-white placeholder-blue-300/50 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed"
              />
            </div>
          </div>

          {/* Password Field */}
          <div>
            <label
              for="password"
              class="block text-sm font-medium text-white mb-2"
            >
              Password
            </label>
            <div class="relative">
              <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                <FiLock class="h-5 w-5 text-yellow-300" aria-hidden="true" />
              </div>
              <input
                type="password"
                id="password"
                name="password"
                required
                minLength={9}
                maxLength={128}
                value={password()}
                disabled={mutation.isPending}
                onInput={(e) => setPassword(e.currentTarget.value)}
                class="block w-full pl-10 pr-3 py-2 bg-blue-900/50 border border-blue-800 rounded-lg text-white placeholder-blue-300/50 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300 disabled:opacity-50 disabled:cursor-not-allowed"
              />
            </div>
            <p class="mt-2 text-sm text-blue-300">
              Must be at least 9 characters long
            </p>
          </div>

          {/* User Role & Status */}
          <div class="flex gap-6">
            <div class="flex-1">
              <label class="block text-sm font-medium text-white mb-2">
                User Role
              </label>
              <div class="relative flex items-center">
                <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <FiShield
                    class="h-5 w-5 text-yellow-300"
                    aria-hidden="true"
                  />
                </div>
                <div class="pl-10 flex items-center">
                  <input
                    type="checkbox"
                    id="is_admin"
                    name="is_admin"
                    checked={isAdmin()}
                    disabled={mutation.isPending}
                    onChange={(e) => setIsAdmin(e.currentTarget.checked)}
                    class="h-4 w-4 rounded border-blue-800 bg-blue-900/50 text-yellow-300 focus:ring-yellow-300/50 focus:ring-offset-2 focus:ring-offset-blue-950 disabled:opacity-50 disabled:cursor-not-allowed"
                  />
                  <label for="is_admin" class="ml-2 text-sm text-white">
                    Administrator
                  </label>
                </div>
              </div>
            </div>

            <div class="flex-1">
              <label class="block text-sm font-medium text-white mb-2">
                Account Status
              </label>
              <div class="relative flex items-center">
                <div class="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <FiUser class="h-5 w-5 text-yellow-300" aria-hidden="true" />
                </div>
                <div class="pl-10 flex items-center">
                  <input
                    type="checkbox"
                    id="is_active"
                    name="is_active"
                    checked={isActive()}
                    disabled={mutation.isPending}
                    onChange={(e) => setIsActive(e.currentTarget.checked)}
                    class="h-4 w-4 rounded border-blue-800 bg-blue-900/50 text-yellow-300 focus:ring-yellow-300/50 focus:ring-offset-2 focus:ring-offset-blue-950 disabled:opacity-50 disabled:cursor-not-allowed"
                  />
                  <label for="is_active" class="ml-2 text-sm text-white">
                    Active Account
                  </label>
                </div>
              </div>
            </div>
          </div>

          {/* Submit Button */}
          <div class="pt-4 flex gap-4">
            <button
              type="button"
              disabled={mutation.isPending}
              onClick={() =>
                navigate({
                  to: "/users",
                  from: Route.fullPath,
                })
              }
              class="flex-1 px-4 py-2 text-sm font-medium text-white bg-blue-900/50 border border-blue-800 rounded-lg hover:bg-blue-900/70 focus:outline-none focus:ring-2 focus:ring-blue-800/50 focus:ring-offset-2 focus:ring-offset-blue-950 transition-colors shadow-sm shadow-black/20 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <span class="flex items-center justify-center gap-2">
                <FiX class="w-4 h-4" />
                Cancel
              </span>
            </button>

            <button
              type="submit"
              disabled={mutation.isPending}
              class="flex-1 px-4 py-2 text-sm font-medium text-blue-950 bg-yellow-300 rounded-lg hover:bg-yellow-400 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:ring-offset-2 focus:ring-offset-blue-950 transition-colors shadow-sm shadow-black/20 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <span class="flex items-center justify-center gap-2">
                <FiSave class="w-4 h-4" />
                {update ? "Update" : "Create"}
              </span>
            </button>
          </div>
        </form>
      </section>
    </div>
  );
}
