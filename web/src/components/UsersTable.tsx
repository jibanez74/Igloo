import { For } from "solid-js";
import { FiMail, FiShield, FiCheck, FiX } from "solid-icons/fi";
import type { SimpleUser } from "../types/User";

type UsersTableProps = {
  users: SimpleUser[];
};

export default function UsersTable(props: UsersTableProps) {
  return (
    <table class="w-full" role="grid">
      <caption class="sr-only">
        List of system users with their details
      </caption>

      <thead>
        <tr class="text-left border-b border-blue-800/20">
          <th scope="col" class="pb-3 text-sm font-medium text-white">
            Name
          </th>
          <th scope="col" class="pb-3 text-sm font-medium text-white">
            Username
          </th>
          <th scope="col" class="pb-3 text-sm font-medium text-white">
            Email
          </th>
          <th scope="col" class="pb-3 text-sm font-medium text-white">
            Status
          </th>
          <th scope="col" class="pb-3 text-sm font-medium text-white">
            Role
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
                  <FiMail class="w-4 h-4" aria-hidden={true} />
                  {user.email}
                </div>
              </td>
              <td class="py-4">
                {user.is_active ? (
                  <span class="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-blue-500/10 text-blue-200">
                    <FiCheck class="w-3 h-3" aria-hidden={true} /> Active
                  </span>
                ) : (
                  <span class="inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-red-400/10 text-red-400">
                    <FiX class="w-3 h-3" aria-hidden={true} /> Inactive
                  </span>
                )}
              </td>
              <td class="py-4">
                {user.is_admin ? (
                  <span class="inline-flex items-center gap-1 text-yellow-300">
                    <FiShield class="w-4 h-4" aria-hidden={true} /> Admin
                  </span>
                ) : (
                  <span class="text-blue-200">User</span>
                )}
              </td>
            </tr>
          )}
        </For>
      </tbody>
    </table>
  );
}
