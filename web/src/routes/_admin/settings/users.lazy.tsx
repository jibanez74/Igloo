import { createLazyFileRoute, Link } from "@tanstack/react-router";
import { useQuery, useQueryClient, useMutation } from "@tanstack/react-query";
import { useState } from "react";
import type { FormEvent } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
import { Users, UserPlus, Pencil, Trash2, KeyRound, ShieldCheck, ShieldOff } from "lucide-react";
import { adminUsersQueryOpts, authUserQueryOpts } from "@/lib/query-opts";
import { ADMIN_USERS_KEY } from "@/lib/constants";
import {
  adminCreateUser,
  adminUpdateUser,
  adminDeleteUser,
  adminResetUserPassword,
} from "@/lib/api";
import { showSuccess, showActionFailed } from "@/lib/toast-helpers";
import type { AdminUserType } from "@/types";

export const Route = createLazyFileRoute("/_admin/settings/users")({
  component: UsersSettings,
});

type DialogState =
  | { type: "none" }
  | { type: "create" }
  | { type: "edit"; user: AdminUserType }
  | { type: "delete"; user: AdminUserType }
  | { type: "reset-password"; user: AdminUserType };

function getInitials(name: string): string {
  const parts = name.trim().split(/\s+/);
  if (parts.length >= 2) {
    return `${parts[0][0]}${parts[parts.length - 1][0]}`.toUpperCase();
  }
  return name[0]?.toUpperCase() ?? "U";
}

function UsersSettings() {
  const queryClient = useQueryClient();
  const { data: usersData, isLoading } = useQuery(adminUsersQueryOpts());
  const { data: authData } = useQuery(authUserQueryOpts());

  const [dialog, setDialog] = useState<DialogState>({ type: "none" });

  const authResolved = authData?.error === false && !!authData.data?.user?.id;
  const currentUserId =
    authData?.error === false ? authData.data?.user?.id : undefined;

  const users: AdminUserType[] =
    usersData?.error === false ? (usersData.data?.users ?? []) : [];

  const closeDialog = () => setDialog({ type: "none" });

  const createMutation = useMutation({
    mutationFn: adminCreateUser,
    onSuccess: res => {
      if (res.error) {
        showActionFailed("create user", res.message);
        return;
      }
      showSuccess("User created successfully");
      queryClient.invalidateQueries({ queryKey: [ADMIN_USERS_KEY] });
      closeDialog();
    },
    onError: () => showActionFailed("create user", "An unexpected error occurred"),
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: { name: string; email: string; is_admin: boolean } }) =>
      adminUpdateUser(id, data),
    onSuccess: res => {
      if (res.error) {
        showActionFailed("update user", res.message);
        return;
      }
      showSuccess("User updated successfully");
      queryClient.invalidateQueries({ queryKey: [ADMIN_USERS_KEY] });
      closeDialog();
    },
    onError: () => showActionFailed("update user", "An unexpected error occurred"),
  });

  const deleteMutation = useMutation({
    mutationFn: adminDeleteUser,
    onSuccess: res => {
      if (res.error) {
        showActionFailed("delete user", res.message);
        return;
      }
      showSuccess("User deleted successfully");
      queryClient.invalidateQueries({ queryKey: [ADMIN_USERS_KEY] });
      closeDialog();
    },
    onError: () => showActionFailed("delete user", "An unexpected error occurred"),
  });

  const resetPasswordMutation = useMutation({
    mutationFn: ({ id, password }: { id: number; password: string }) =>
      adminResetUserPassword(id, password),
    onSuccess: res => {
      if (res.error) {
        showActionFailed("reset password", res.message);
        return;
      }
      showSuccess("Password reset successfully");
      closeDialog();
    },
    onError: () => showActionFailed("reset password", "An unexpected error occurred"),
  });

  return (
    <div className="space-y-8">
      <Card className="border-slate-700/50 bg-slate-800/30">
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle className="flex items-center gap-2 text-white">
              <Users className="size-5 text-amber-400" aria-hidden="true" />
              User Management
            </CardTitle>
            <CardDescription className="text-slate-300">
              Create, edit, and remove user accounts
            </CardDescription>
          </div>
          <Button
            variant="accent"
            onClick={() => setDialog({ type: "create" })}
            className="shrink-0"
          >
            <UserPlus className="size-4" aria-hidden="true" />
            Add User
          </Button>
        </CardHeader>

        <CardContent>
          {isLoading && (
            <p className="text-slate-300">Loading users...</p>
          )}

          {!isLoading && usersData?.error && (
            <p className="text-red-400">
              {usersData.message || "Failed to load users"}
            </p>
          )}

          {!isLoading && !usersData?.error && users.length === 0 && (
            <p className="text-slate-400">No users found.</p>
          )}

          {!isLoading && users.length > 0 && (
            <ul className="divide-y divide-slate-700/50" role="list" aria-label="User list">
              {users.map(user => (
                <li key={user.id} className="flex items-center gap-4 py-4">
                  <Avatar className="size-10 shrink-0">
                    {user.avatar && (
                      <AvatarImage src={user.avatar} alt={user.name} />
                    )}
                    <AvatarFallback className="bg-amber-500/20 text-amber-400">
                      {getInitials(user.name)}
                    </AvatarFallback>
                  </Avatar>

                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium text-white">
                      {user.name}
                      {authResolved && user.id === currentUserId && (
                        <span className="ml-2 text-xs text-slate-400">(you)</span>
                      )}
                    </p>
                    <p className="truncate text-xs text-slate-400">{user.email}</p>
                  </div>

                  <div className="shrink-0">
                    {user.is_admin ? (
                      <span
                        className="inline-flex items-center gap-1 rounded-full bg-amber-500/10 px-2 py-0.5 text-xs font-medium text-amber-400"
                        aria-label="Admin"
                      >
                        <ShieldCheck className="size-3" aria-hidden="true" />
                        Admin
                      </span>
                    ) : (
                      <span
                        className="inline-flex items-center gap-1 rounded-full bg-slate-700/50 px-2 py-0.5 text-xs font-medium text-slate-400"
                        aria-label="User"
                      >
                        <ShieldOff className="size-3" aria-hidden="true" />
                        User
                      </span>
                    )}
                  </div>

                  <div className="flex shrink-0 items-center gap-2">
                    {authResolved && user.id === currentUserId ? (
                      <Link
                        to="/settings/account"
                        className="text-xs text-slate-400 underline-offset-2 hover:text-amber-400 hover:underline"
                      >
                        Account settings
                      </Link>
                    ) : (
                      <>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => setDialog({ type: "reset-password", user })}
                          aria-label={`Reset password for ${user.name}`}
                          className="text-slate-400 hover:text-white"
                        >
                          <KeyRound className="size-4" aria-hidden="true" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => setDialog({ type: "edit", user })}
                          aria-label={`Edit ${user.name}`}
                          className="text-slate-400 hover:text-white"
                        >
                          <Pencil className="size-4" aria-hidden="true" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => setDialog({ type: "delete", user })}
                          aria-label={`Delete ${user.name}`}
                          className="text-slate-400 hover:text-red-400"
                        >
                          <Trash2 className="size-4" aria-hidden="true" />
                        </Button>
                      </>
                    )}
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      {dialog.type === "create" && (
        <CreateUserDialog
          onClose={closeDialog}
          onSubmit={data => createMutation.mutate(data)}
          isPending={createMutation.isPending}
        />
      )}

      {dialog.type === "edit" && (
        <EditUserDialog
          user={dialog.user}
          onClose={closeDialog}
          onSubmit={data => updateMutation.mutate({ id: dialog.user.id, data })}
          isPending={updateMutation.isPending}
        />
      )}

      {dialog.type === "delete" && (
        <DeleteUserDialog
          user={dialog.user}
          onClose={closeDialog}
          onConfirm={() => deleteMutation.mutate(dialog.user.id)}
          isPending={deleteMutation.isPending}
        />
      )}

      {dialog.type === "reset-password" && (
        <ResetPasswordDialog
          user={dialog.user}
          onClose={closeDialog}
          onSubmit={password =>
            resetPasswordMutation.mutate({ id: dialog.user.id, password })
          }
          isPending={resetPasswordMutation.isPending}
        />
      )}
    </div>
  );
}

type CreateUserDialogProps = {
  onClose: () => void;
  onSubmit: (data: { name: string; email: string; password: string; is_admin: boolean }) => void;
  isPending: boolean;
};

function CreateUserDialog({ onClose, onSubmit, isPending }: CreateUserDialogProps) {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [isAdmin, setIsAdmin] = useState(false);

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    onSubmit({ name: name.trim(), email: email.trim(), password, is_admin: isAdmin });
  };

  return (
    <Dialog open onOpenChange={open => { if (!open) onClose(); }}>
      <DialogContent className="border-slate-700 bg-slate-900 text-white sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="text-white">Add User</DialogTitle>
          <DialogDescription className="text-slate-300">
            Create a new user account.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="create-name" className="text-slate-300">Name</Label>
            <Input
              id="create-name"
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="Full name"
              maxLength={100}
              required
              aria-label="User name"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="create-email" className="text-slate-300">Email</Label>
            <Input
              id="create-email"
              type="email"
              value={email}
              onChange={e => setEmail(e.target.value)}
              placeholder="user@example.com"
              required
              aria-label="User email"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="create-password" className="text-slate-300">Password</Label>
            <Input
              id="create-password"
              type="password"
              value={password}
              onChange={e => setPassword(e.target.value)}
              placeholder="At least 9 characters"
              required
              aria-label="User password"
            />
            <p className="text-xs text-slate-400">Must be 9–128 characters</p>
          </div>

          <div className="flex items-center gap-3">
            <input
              id="create-is-admin"
              type="checkbox"
              checked={isAdmin}
              onChange={e => setIsAdmin(e.target.checked)}
              className="size-4 rounded border-slate-600 bg-slate-800 accent-amber-500"
              aria-label="Grant admin privileges"
            />
            <Label htmlFor="create-is-admin" className="cursor-pointer text-slate-300">
              Grant admin privileges
            </Label>
          </div>

          <DialogFooter className="gap-2 pt-2 sm:gap-0">
            <Button type="button" variant="outline" onClick={onClose} disabled={isPending}>
              Cancel
            </Button>
            <Button type="submit" variant="accent" disabled={isPending}>
              {isPending ? "Creating..." : "Create User"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

type EditUserDialogProps = {
  user: AdminUserType;
  onClose: () => void;
  onSubmit: (data: { name: string; email: string; is_admin: boolean }) => void;
  isPending: boolean;
};

function EditUserDialog({ user, onClose, onSubmit, isPending }: EditUserDialogProps) {
  const [name, setName] = useState(user.name);
  const [email, setEmail] = useState(user.email);
  const [isAdmin, setIsAdmin] = useState(user.is_admin);

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    onSubmit({ name: name.trim(), email: email.trim(), is_admin: isAdmin });
  };

  return (
    <Dialog open onOpenChange={open => { if (!open) onClose(); }}>
      <DialogContent className="border-slate-700 bg-slate-900 text-white sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="text-white">Edit User</DialogTitle>
          <DialogDescription className="text-slate-300">
            Update account information for {user.name}.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="edit-name" className="text-slate-300">Name</Label>
            <Input
              id="edit-name"
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="Full name"
              maxLength={100}
              required
              aria-label="User name"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="edit-email" className="text-slate-300">Email</Label>
            <Input
              id="edit-email"
              type="email"
              value={email}
              onChange={e => setEmail(e.target.value)}
              placeholder="user@example.com"
              required
              aria-label="User email"
            />
          </div>

          <div className="flex items-center gap-3">
            <input
              id="edit-is-admin"
              type="checkbox"
              checked={isAdmin}
              onChange={e => setIsAdmin(e.target.checked)}
              className="size-4 rounded border-slate-600 bg-slate-800 accent-amber-500"
              aria-label="Admin privileges"
            />
            <Label htmlFor="edit-is-admin" className="cursor-pointer text-slate-300">
              Admin privileges
            </Label>
          </div>

          <DialogFooter className="gap-2 pt-2 sm:gap-0">
            <Button type="button" variant="outline" onClick={onClose} disabled={isPending}>
              Cancel
            </Button>
            <Button type="submit" variant="accent" disabled={isPending}>
              {isPending ? "Saving..." : "Save Changes"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

type DeleteUserDialogProps = {
  user: AdminUserType;
  onClose: () => void;
  onConfirm: () => void;
  isPending: boolean;
};

function DeleteUserDialog({ user, onClose, onConfirm, isPending }: DeleteUserDialogProps) {
  const [confirmText, setConfirmText] = useState("");

  return (
    <Dialog open onOpenChange={open => { if (!open) onClose(); }}>
      <DialogContent className="border-slate-700 bg-slate-900 text-white sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="text-red-400">Delete User</DialogTitle>
          <DialogDescription className="text-slate-300">
            This will permanently delete <strong className="text-white">{user.name}</strong> ({user.email}).
            This action cannot be undone.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="delete-confirm" className="text-slate-300">
              Type <span className="font-mono font-bold">DELETE</span> to confirm:
            </Label>
            <Input
              id="delete-confirm"
              value={confirmText}
              onChange={e => setConfirmText(e.target.value)}
              placeholder="DELETE"
              className="font-mono"
              aria-label="Type DELETE to confirm user deletion"
            />
          </div>
        </div>

        <DialogFooter className="gap-2 sm:gap-0">
          <Button variant="outline" onClick={onClose} disabled={isPending}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            onClick={onConfirm}
            disabled={isPending || confirmText !== "DELETE"}
          >
            {isPending ? "Deleting..." : "Delete User"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

type ResetPasswordDialogProps = {
  user: AdminUserType;
  onClose: () => void;
  onSubmit: (password: string) => void;
  isPending: boolean;
};

function ResetPasswordDialog({ user, onClose, onSubmit, isPending }: ResetPasswordDialogProps) {
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (password !== confirmPassword) return;
    onSubmit(password);
  };

  const mismatch = confirmPassword.length > 0 && password !== confirmPassword;

  return (
    <Dialog open onOpenChange={open => { if (!open) onClose(); }}>
      <DialogContent className="border-slate-700 bg-slate-900 text-white sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="text-white">Reset Password</DialogTitle>
          <DialogDescription className="text-slate-300">
            Set a new password for <strong className="text-white">{user.name}</strong>.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor="reset-password" className="text-slate-300">New Password</Label>
            <Input
              id="reset-password"
              type="password"
              value={password}
              onChange={e => setPassword(e.target.value)}
              placeholder="At least 9 characters"
              required
              aria-label="New password"
            />
            <p className="text-xs text-slate-400">Must be 9–128 characters</p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="reset-confirm-password" className="text-slate-300">Confirm Password</Label>
            <Input
              id="reset-confirm-password"
              type="password"
              value={confirmPassword}
              onChange={e => setConfirmPassword(e.target.value)}
              placeholder="Repeat new password"
              required
              aria-label="Confirm new password"
              aria-invalid={mismatch}
            />
            {mismatch && (
              <p className="text-xs text-red-400" role="alert">
                Passwords do not match
              </p>
            )}
          </div>

          <DialogFooter className="gap-2 pt-2 sm:gap-0">
            <Button type="button" variant="outline" onClick={onClose} disabled={isPending}>
              Cancel
            </Button>
            <Button
              type="submit"
              variant="accent"
              disabled={isPending || password.length < 9 || mismatch}
            >
              {isPending ? "Resetting..." : "Reset Password"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
