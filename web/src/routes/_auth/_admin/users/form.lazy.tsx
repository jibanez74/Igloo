import { createLazyFileRoute } from "@tanstack/solid-router";
import { FiUser, FiMail, FiLock, FiShield } from "solid-icons/fi";

export const Route = createLazyFileRoute("/_auth/_admin/users/form")({
  component: UserFormPage,
});

function UserFormPage() {
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

        <form class="space-y-6">
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
                class="block w-full pl-10 pr-3 py-2 bg-blue-900/50 border border-blue-800 rounded-lg text-white placeholder-blue-300/50 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300"
                placeholder="John Doe"
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
                class="block w-full pl-10 pr-3 py-2 bg-blue-900/50 border border-blue-800 rounded-lg text-white placeholder-blue-300/50 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300"
                placeholder="johndoe"
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
                class="block w-full pl-10 pr-3 py-2 bg-blue-900/50 border border-blue-800 rounded-lg text-white placeholder-blue-300/50 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300"
                placeholder="john@example.com"
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
                class="block w-full pl-10 pr-3 py-2 bg-blue-900/50 border border-blue-800 rounded-lg text-white placeholder-blue-300/50 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:border-yellow-300"
                placeholder="••••••••"
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
                  <FiShield class="h-5 w-5 text-yellow-300" aria-hidden="true" />
                </div>
                <div class="pl-10 flex items-center">
                  <input
                    type="checkbox"
                    id="is_admin"
                    name="is_admin"
                    class="h-4 w-4 rounded border-blue-800 bg-blue-900/50 text-yellow-300 focus:ring-yellow-300/50 focus:ring-offset-2 focus:ring-offset-blue-950"
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
                    checked
                    class="h-4 w-4 rounded border-blue-800 bg-blue-900/50 text-yellow-300 focus:ring-yellow-300/50 focus:ring-offset-2 focus:ring-offset-blue-950"
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
              class="flex-1 px-4 py-2 text-sm font-medium text-white bg-blue-900/50 border border-blue-800 rounded-lg hover:bg-blue-900/70 focus:outline-none focus:ring-2 focus:ring-blue-800/50 focus:ring-offset-2 focus:ring-offset-blue-950 transition-colors shadow-sm shadow-black/20"
            >
              Cancel
            </button>
            <button
              type="submit"
              class="flex-1 px-4 py-2 text-sm font-medium text-blue-950 bg-yellow-300 rounded-lg hover:bg-yellow-400 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:ring-offset-2 focus:ring-offset-blue-950 transition-colors shadow-sm shadow-black/20"
            >
              Create User
            </button>
          </div>
        </form>
      </section>
    </div>
  );
}
