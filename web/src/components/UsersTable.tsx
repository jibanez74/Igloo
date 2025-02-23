import { FiMail, FiShield, FiCheck, FiX } from "react-icons/fi";
import type { User } from "@/types/User";

type UsersTableProps = {
  users: User[];
};

export default function UsersTable({ users }: UsersTableProps) {
  return (
    <table className='w-full' role='grid'>
      <caption className='sr-only'>
        List of system users with their details
      </caption>
      <thead>
        <tr className='text-left border-b border-sky-200/10'>
          <th scope='col' className='pb-3 text-sm font-medium text-sky-200'>
            Name
          </th>
          <th scope='col' className='pb-3 text-sm font-medium text-sky-200'>
            Username
          </th>
          <th scope='col' className='pb-3 text-sm font-medium text-sky-200'>
            Email
          </th>
          <th scope='col' className='pb-3 text-sm font-medium text-sky-200'>
            Status
          </th>
          <th scope='col' className='pb-3 text-sm font-medium text-sky-200'>
            Role
          </th>
        </tr>
      </thead>
      <tbody className='divide-y divide-sky-200/10'>
        {users.map(user => (
          <tr key={user.id} className='hover:bg-sky-500/5 transition-colors'>
            <th scope='row' className='py-4 text-white font-normal'>
              {user.name}
            </th>
            <td className='py-4 text-sky-200'>{user.username}</td>
            <td className='py-4 text-sky-200'>
              <div className='flex items-center gap-2'>
                <FiMail className='w-4 h-4' aria-hidden='true' />
                {user.email}
              </div>
            </td>
            <td className='py-4'>
              {user.is_active ? (
                <span className='inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-emerald-500/10 text-emerald-400'>
                  <FiCheck className='w-3 h-3' aria-hidden='true' /> Active
                </span>
              ) : (
                <span className='inline-flex items-center gap-1 px-2 py-1 rounded-full text-xs font-medium bg-rose-500/10 text-rose-400'>
                  <FiX className='w-3 h-3' aria-hidden='true' /> Inactive
                </span>
              )}
            </td>
            <td className='py-4'>
              {user.is_admin ? (
                <span className='inline-flex items-center gap-1 text-amber-300'>
                  <FiShield className='w-4 h-4' aria-hidden='true' /> Admin
                </span>
              ) : (
                <span className='text-sky-200'>User</span>
              )}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
