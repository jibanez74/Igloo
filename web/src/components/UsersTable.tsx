import { For } from "solid-js";
import { useRouter } from "@tanstack/solid-router";
import {
  FiMail,
  FiShield,
  FiCheck,
  FiX,
  FiEdit2,
  FiTrash2,
} from "solid-icons/fi";
import type { SimpleUser } from "../types/User";

type UsersTableProps = {
  users: SimpleUser[];
};

export default function UsersTable(props: UsersTableProps) {
  const router = useRouter();

  return (
    <div role="region" aria-label="Users" class="overflow-x-auto">
      <table 
        class="w-full" 
        role="grid"
        aria-describedby="users-table-description"
      >
        <caption id="users-table-description" class="sr-only">
          List of system users with their details including name, username, email, status, role, and available actions
        </caption>

        <thead>
          <tr class="text-left border-b border-blue-800/20">
            <th scope="col" class="pb-3 text-sm font-medium text-white">
              <span class="sr-only">User </span>Name
            </th>
            <th scope="col" class="pb-3 text-sm font-medium text-white">
              Username
            </th>
            <th scope="col" class="pb-3 text-sm font-medium text-white">
              Email Address
            </th>
            <th scope="col" class="pb-3 text-sm font-medium text-white">
              Account Status
            </th>
            <th scope="col" class="pb-3 text-sm font-medium text-white">
              User Role
            </th>
            <th scope="col" class="pb-3 text-sm font-medium text-white">
              <span class="sr-only">User </span>Actions
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-blue-800/20">
          <For each={props.users}>
            {(user) => (
              <tr class="hover:bg-blue-800/20 transition-colors">
                <th scope="row" class="py-4 text-white font-normal">
                  {user.name}
                </th>
                <td class="py-4 text-blue-200">{user.username}</td>
                <td class="py-4 text-blue-200">
                  <div class="flex items-center gap-2">
                    <FiMail 
                      class="w-4 h-4" 
                      aria-hidden="true"
                      role="presentation" 
                    />
                    <span>{user.email}</span>
                  </div>
                </td>
                <td class="py-4">
                  {user.is_active ? (
                    <span 
                      class="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-blue-500/10 text-blue-200"
                      role="status"
                      aria-label="User account is active"
                    >
                      <FiCheck 
                        class="w-3 h-3" 
                        aria-hidden="true"
                        role="presentation" 
                      /> 
                      Active
                    </span>
                  ) : (
                    <span 
                      class="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-red-400/10 text-red-400"
                      role="status"
                      aria-label="User account is inactive"
                    >
                      <FiX 
                        class="w-3 h-3" 
                        aria-hidden="true"
                        role="presentation" 
                      /> 
                      Inactive
                    </span>
                  )}
                </td>
                <td class="py-4">
                  {user.is_admin ? (
                    <span 
                      class="inline-flex items-center gap-1 text-yellow-300"
                      role="status"
                      aria-label="User has administrator privileges"
                    >
                      <FiShield 
                        class="w-4 h-4" 
                        aria-hidden="true"
                        role="presentation" 
                      /> 
                      Admin
                    </span>
                  ) : (
                    <span 
                      class="text-blue-200"
                      role="status"
                      aria-label="User has standard privileges"
                    >
                      User
                    </span>
                  )}
                </td>
                <td class="py-4">
                  <div 
                    class="flex items-center gap-2"
                    role="group"
                    aria-label={`Actions for ${user.name}`}
                  >
                    <button
                      type="button"
                      onClick={() =>
                        router.navigate({
                          to: "/users/form",
                          search: {
                            id: user.id,
                            update: true,
                          },
                        })
                      }
                      class="p-1.5 text-blue-200 hover:text-yellow-300 rounded-lg hover:bg-blue-800/50 focus:outline-none focus:ring-2 focus:ring-yellow-300/50 focus:ring-offset-2 focus:ring-offset-blue-950 transition-colors"
                      aria-label={`Edit user ${user.name}`}
                    >
                      <FiEdit2 
                        class="w-4 h-4" 
                        aria-hidden="true"
                        role="presentation" 
                      />
                    </button>

                    <button
                      type="button"
                      class="p-1.5 text-blue-200 hover:text-red-400 rounded-lg hover:bg-blue-800/50 focus:outline-none focus:ring-2 focus:ring-red-400/50 focus:ring-offset-2 focus:ring-offset-blue-950 transition-colors"
                      aria-label={`Delete user ${user.name}`}
                    >
                      <FiTrash2 
                        class="w-4 h-4" 
                        aria-hidden="true"
                        role="presentation" 
                      />
                    </button>
                  </div>
                </td>
              </tr>
            )}
          </For>
        </tbody>
      </table>
    </div>
  );
}
