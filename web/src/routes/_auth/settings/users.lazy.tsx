import { createLazyFileRoute, Link } from "@tanstack/react-router";
import { useQuery, useQueryClient, useMutation } from "@tanstack/react-query";
import { useId, useRef, useState } from "react";
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
import { lightInputClassName } from "@/lib/input-styles";
import { showSuccess, showActionFailed, showValidationError } from "@/lib/toast-helpers";
import type { AdminUserType } from "@/types";
import { useDialogFocusRestore } from "@/hooks/useDialogFocusRestore";

export const Route = createLazyFileRoute("/_auth/settings/users")({
  component: UsersSettings,
});

type DialogState =
  | { type: "none" }
  | { type: "create" }
  | { type: "edit"; user: AdminUserType }
  | { type: "delete"; user: AdminUserType }
  | { type: "reset-password"; user: AdminUserType };

type UserFormErrorField = "name" | "email" | "password" | "confirmPassword" | "form";
type UserFormErrors = Partial<Record<UserFormErrorField, string>>;
type DialogCloseAutoFocusHandler = (event: Event) => void;

const MIN_PASSWORD_LENGTH = 9;
const MAX_PASSWORD_LENGTH = 128;

function describedBy(...ids: Array<string | false | null | undefined>) {
  const value = ids.filter(Boolean).join(" ");
  return value || undefined;
}

function isValidEmail(email: string) {
  return /^[^\s@]+@[^\s@]+$/.test(email);
}

function hasDuplicateEmail(
  users: AdminUserType[],
  email: string,
  ignoredUserId?: number,
) {
  const normalizedEmail = email.trim().toLowerCase();
  return users.some(user => {
    if (ignoredUserId !== undefined && user.id === ignoredUserId) {
      return false;
    }
    return user.email.trim().toLowerCase() === normalizedEmail;
  });
}

function firstErrorMessage(errors: UserFormErrors) {
  return (
    errors.name ??
    errors.email ??
    errors.password ??
    errors.confirmPassword ??
    errors.form ??
    "Check the form for errors."
  );
}

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
  const [dialogError, setDialogError] = useState("");
  const addUserButtonRef = useRef<HTMLButtonElement | null>(null);
  const {
    setRestoreFocusTarget,
    restoreFocus,
    onCloseAutoFocus: handleDialogCloseAutoFocus,
  } = useDialogFocusRestore({ fallbackRef: addUserButtonRef });

  const authResolved = authData?.error === false && !!authData.data?.user?.id;
  const currentUserId =
    authData?.error === false ? authData.data?.user?.id : undefined;

  const users: AdminUserType[] =
    usersData?.error === false ? (usersData.data?.users ?? []) : [];

  const openDialog = (nextDialog: DialogState, restoreTarget: HTMLElement) => {
    setDialogError("");
    setRestoreFocusTarget(restoreTarget);
    setDialog(nextDialog);
  };

  const closeDialog = () => {
    setDialog({ type: "none" });
    setDialogError("");
    restoreFocus();
  };

  const createMutation = useMutation({
    mutationFn: adminCreateUser,
    onSuccess: res => {
      if (res.error) {
        setDialogError(res.message);
        showActionFailed("create user", res.message);
        return;
      }
      showSuccess("User created successfully");
      queryClient.invalidateQueries({ queryKey: [ADMIN_USERS_KEY] });
      closeDialog();
    },
    onError: () => {
      const message = "An unexpected error occurred";
      setDialogError(message);
      showActionFailed("create user", message);
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: number; data: { name: string; email: string; is_admin: boolean } }) =>
      adminUpdateUser(id, data),
    onSuccess: res => {
      if (res.error) {
        setDialogError(res.message);
        showActionFailed("update user", res.message);
        return;
      }
      showSuccess("User updated successfully");
      queryClient.invalidateQueries({ queryKey: [ADMIN_USERS_KEY] });
      closeDialog();
    },
    onError: () => {
      const message = "An unexpected error occurred";
      setDialogError(message);
      showActionFailed("update user", message);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: adminDeleteUser,
    onSuccess: res => {
      if (res.error) {
        setDialogError(res.message);
        showActionFailed("delete user", res.message);
        return;
      }
      showSuccess("User deleted successfully");
      queryClient.invalidateQueries({ queryKey: [ADMIN_USERS_KEY] });
      closeDialog();
    },
    onError: () => {
      const message = "An unexpected error occurred";
      setDialogError(message);
      showActionFailed("delete user", message);
    },
  });

  const resetPasswordMutation = useMutation({
    mutationFn: ({ id, password }: { id: number; password: string }) =>
      adminResetUserPassword(id, password),
    onSuccess: res => {
      if (res.error) {
        setDialogError(res.message);
        showActionFailed("reset password", res.message);
        return;
      }
      showSuccess("Password reset successfully");
      closeDialog();
    },
    onError: () => {
      const message = "An unexpected error occurred";
      setDialogError(message);
      showActionFailed("reset password", message);
    },
  });

  return (
    <div className="space-y-8">
      <Card className="border-border/50 bg-muted/30">
        <CardHeader className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <CardTitle asChild className="flex items-center gap-2 text-foreground">
              <h2>
                <Users className="size-5 text-primary" aria-hidden="true" />
                User Management
              </h2>
            </CardTitle>
            <CardDescription className="text-muted-foreground">
              Create, edit, and remove user accounts
            </CardDescription>
          </div>
          <Button
            ref={addUserButtonRef}
            variant="accent"
            onClick={event => openDialog({ type: "create" }, event.currentTarget)}
            className="w-full shrink-0 sm:w-auto"
          >
            <UserPlus className="size-4" aria-hidden="true" />
            Add User
          </Button>
        </CardHeader>

        <CardContent>
          {isLoading && (
            <p className="text-muted-foreground">Loading users...</p>
          )}

          {!isLoading && usersData?.error && (
            <p className="text-destructive">
              {usersData.message || "Failed to load users"}
            </p>
          )}

          {!isLoading && !usersData?.error && users.length === 0 && (
            <p className="text-muted-foreground">No users found.</p>
          )}

          {!isLoading && users.length > 0 && (
            <ul className="divide-y divide-border" role="list" aria-label="User list">
              {users.map(user => (
                <li key={user.id} className="flex flex-col gap-3 py-4 sm:flex-row sm:items-center sm:gap-4">
                  <div className="flex min-w-0 items-center gap-3 sm:flex-1">
                    <Avatar className="size-10 shrink-0">
                      {user.avatar && (
                        <AvatarImage src={user.avatar} alt={user.name} />
                      )}
                      <AvatarFallback className="bg-primary/20 text-primary">
                        {getInitials(user.name)}
                      </AvatarFallback>
                    </Avatar>

                    <div className="min-w-0 flex-1">
                      <p className="truncate text-sm font-medium text-foreground">
                        {user.name}
                        {authResolved && user.id === currentUserId && (
                          <span className="ml-2 text-xs text-muted-foreground">(you)</span>
                        )}
                      </p>
                      <p className="truncate text-xs text-muted-foreground">{user.email}</p>
                    </div>
                  </div>

                  <div className="flex flex-wrap items-center justify-between gap-3 sm:shrink-0 sm:justify-end">
                    <div className="shrink-0">
                      {user.is_admin ? (
                        <span
                          className="inline-flex items-center gap-1 rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-primary"
                        >
                          <ShieldCheck className="size-3" aria-hidden="true" />
                          Admin
                        </span>
                      ) : (
                        <span
                          className="inline-flex items-center gap-1 rounded-full bg-accent/50 px-2 py-0.5 text-xs font-medium text-muted-foreground"
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
                          className="text-xs text-muted-foreground underline-offset-2 hover:text-primary hover:underline"
                        >
                          Account settings
                        </Link>
                      ) : (
                        <>
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={event =>
                              openDialog(
                                { type: "reset-password", user },
                                event.currentTarget,
                              )
                            }
                            aria-label={`Reset password for ${user.name}`}
                            className="text-muted-foreground hover:text-foreground"
                          >
                            <KeyRound className="size-4" aria-hidden="true" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={event =>
                              openDialog({ type: "edit", user }, event.currentTarget)
                            }
                            aria-label={`Edit ${user.name}`}
                            className="text-muted-foreground hover:text-foreground"
                          >
                            <Pencil className="size-4" aria-hidden="true" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={event =>
                              openDialog({ type: "delete", user }, event.currentTarget)
                            }
                            aria-label={`Delete ${user.name}`}
                            className="text-muted-foreground hover:text-destructive"
                          >
                            <Trash2 className="size-4" aria-hidden="true" />
                          </Button>
                        </>
                      )}
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      {dialog.type === "create" && (
        <CreateUserDialog
          users={users}
          onClose={closeDialog}
          onSubmit={data => createMutation.mutate(data)}
          isPending={createMutation.isPending}
          serverError={dialogError}
          onClearServerError={() => setDialogError("")}
          onCloseAutoFocus={handleDialogCloseAutoFocus}
        />
      )}

      {dialog.type === "edit" && (
        <EditUserDialog
          key={dialog.user.id}
          user={dialog.user}
          users={users}
          onClose={closeDialog}
          onSubmit={data => updateMutation.mutate({ id: dialog.user.id, data })}
          isPending={updateMutation.isPending}
          serverError={dialogError}
          onClearServerError={() => setDialogError("")}
          onCloseAutoFocus={handleDialogCloseAutoFocus}
        />
      )}

      {dialog.type === "delete" && (
        <DeleteUserDialog
          user={dialog.user}
          onClose={closeDialog}
          onConfirm={() => deleteMutation.mutate(dialog.user.id)}
          isPending={deleteMutation.isPending}
          serverError={dialogError}
          onClearServerError={() => setDialogError("")}
          onCloseAutoFocus={handleDialogCloseAutoFocus}
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
          serverError={dialogError}
          onClearServerError={() => setDialogError("")}
          onCloseAutoFocus={handleDialogCloseAutoFocus}
        />
      )}
    </div>
  );
}

type CreateUserDialogProps = {
  users: AdminUserType[];
  onClose: () => void;
  onSubmit: (data: { name: string; email: string; password: string; is_admin: boolean }) => void;
  isPending: boolean;
  serverError: string;
  onClearServerError: () => void;
  onCloseAutoFocus: DialogCloseAutoFocusHandler;
};

function CreateUserDialog({
  users,
  onClose,
  onSubmit,
  isPending,
  serverError,
  onClearServerError,
  onCloseAutoFocus,
}: CreateUserDialogProps) {
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [isAdmin, setIsAdmin] = useState(false);
  const [errors, setErrors] = useState<UserFormErrors>({});
  const nameId = useId();
  const emailId = useId();
  const passwordId = useId();
  const isAdminId = useId();
  const nameErrorId = `${nameId}-error`;
  const emailErrorId = `${emailId}-error`;
  const passwordDescriptionId = `${passwordId}-description`;
  const passwordErrorId = `${passwordId}-error`;
  const formErrorId = `${passwordId}-form-error`;

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    const trimmedName = name.trim();
    const trimmedEmail = email.trim();
    const nextErrors: UserFormErrors = {};

    if (trimmedName === "") {
      nextErrors.name = "Name is required.";
    } else if (trimmedName.length > 100) {
      nextErrors.name = "Name must be 100 characters or less.";
    }

    if (trimmedEmail === "") {
      nextErrors.email = "Email is required.";
    } else if (!isValidEmail(trimmedEmail)) {
      nextErrors.email = "Enter a valid email address.";
    } else if (hasDuplicateEmail(users, trimmedEmail)) {
      nextErrors.email = "A user with that email already exists.";
    }

    if (password.length < MIN_PASSWORD_LENGTH) {
      nextErrors.password = "Password must be at least 9 characters.";
    } else if (password.length > MAX_PASSWORD_LENGTH) {
      nextErrors.password = "Password must be 128 characters or less.";
    }

    if (Object.keys(nextErrors).length > 0) {
      setErrors(nextErrors);
      showValidationError(firstErrorMessage(nextErrors));
      if (nextErrors.name) {
        document.getElementById(nameId)?.focus();
      } else if (nextErrors.email) {
        document.getElementById(emailId)?.focus();
      } else {
        document.getElementById(passwordId)?.focus();
      }
      return;
    }

    setErrors({});
    onClearServerError();
    onSubmit({
      name: trimmedName,
      email: trimmedEmail,
      password,
      is_admin: isAdmin,
    });
  };

  return (
    <Dialog open onOpenChange={open => { if (!open) onClose(); }}>
      <DialogContent
        className="border-border bg-card text-foreground sm:max-w-md"
        onCloseAutoFocus={onCloseAutoFocus}
      >
        <DialogHeader>
          <DialogTitle className="text-foreground">Add User</DialogTitle>
          <DialogDescription className="text-muted-foreground">
            Create a new user account.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} noValidate className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor={nameId} className="text-muted-foreground">Name</Label>
            <Input
              id={nameId}
              name="name"
              value={name}
              onChange={e => {
                setName(e.target.value);
                setErrors(current => ({ ...current, name: undefined }));
                onClearServerError();
              }}
              placeholder="Full name"
              maxLength={100}
              required
              aria-required="true"
              aria-invalid={!!errors.name || undefined}
              aria-describedby={describedBy(errors.name && nameErrorId)}
              className={lightInputClassName}
              aria-label="User name"
            />
            {errors.name && (
              <p id={nameErrorId} className="text-xs text-destructive" role="alert">
                {errors.name}
              </p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor={emailId} className="text-muted-foreground">Email</Label>
            <Input
              id={emailId}
              name="email"
              type="email"
              value={email}
              onChange={e => {
                setEmail(e.target.value);
                setErrors(current => ({ ...current, email: undefined }));
                onClearServerError();
              }}
              placeholder="user@example.com"
              required
              aria-required="true"
              aria-invalid={!!errors.email || undefined}
              aria-describedby={describedBy(errors.email && emailErrorId)}
              className={lightInputClassName}
              aria-label="User email"
            />
            {errors.email && (
              <p id={emailErrorId} className="text-xs text-destructive" role="alert">
                {errors.email}
              </p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor={passwordId} className="text-muted-foreground">Password</Label>
            <Input
              id={passwordId}
              name="password"
              type="password"
              value={password}
              onChange={e => {
                setPassword(e.target.value);
                setErrors(current => ({ ...current, password: undefined }));
                onClearServerError();
              }}
              placeholder="At least 9 characters"
              required
              aria-required="true"
              aria-invalid={!!errors.password || undefined}
              aria-describedby={describedBy(
                passwordDescriptionId,
                errors.password && passwordErrorId,
              )}
              className={lightInputClassName}
              aria-label="User password"
            />
            <p id={passwordDescriptionId} className="text-xs text-muted-foreground">
              Must be 9–128 characters
            </p>
            {errors.password && (
              <p id={passwordErrorId} className="text-xs text-destructive" role="alert">
                {errors.password}
              </p>
            )}
          </div>

          <div className="flex items-center gap-3">
            <input
              id={isAdminId}
              type="checkbox"
              checked={isAdmin}
              onChange={e => setIsAdmin(e.target.checked)}
              className="size-4 rounded-sm border-border bg-muted accent-primary"
              aria-label="Grant admin privileges"
            />
            <Label htmlFor={isAdminId} className="cursor-pointer text-muted-foreground">
              Grant admin privileges
            </Label>
          </div>

          {serverError && (
            <p id={formErrorId} className="text-sm text-destructive" role="alert">
              {serverError}
            </p>
          )}

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
  users: AdminUserType[];
  onClose: () => void;
  onSubmit: (data: { name: string; email: string; is_admin: boolean }) => void;
  isPending: boolean;
  serverError: string;
  onClearServerError: () => void;
  onCloseAutoFocus: DialogCloseAutoFocusHandler;
};

function EditUserDialog({
  user,
  users,
  onClose,
  onSubmit,
  isPending,
  serverError,
  onClearServerError,
  onCloseAutoFocus,
}: EditUserDialogProps) {
  const [name, setName] = useState(user.name);
  const [email, setEmail] = useState(user.email);
  const [isAdmin, setIsAdmin] = useState(user.is_admin);
  const [errors, setErrors] = useState<UserFormErrors>({});
  const nameId = useId();
  const emailId = useId();
  const isAdminId = useId();
  const nameErrorId = `${nameId}-error`;
  const emailErrorId = `${emailId}-error`;
  const formErrorId = `${emailId}-form-error`;

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    const trimmedName = name.trim();
    const trimmedEmail = email.trim();
    const nextErrors: UserFormErrors = {};

    if (trimmedName === "") {
      nextErrors.name = "Name is required.";
    } else if (trimmedName.length > 100) {
      nextErrors.name = "Name must be 100 characters or less.";
    }

    if (trimmedEmail === "") {
      nextErrors.email = "Email is required.";
    } else if (!isValidEmail(trimmedEmail)) {
      nextErrors.email = "Enter a valid email address.";
    } else if (hasDuplicateEmail(users, trimmedEmail, user.id)) {
      nextErrors.email = "A user with that email already exists.";
    }

    if (Object.keys(nextErrors).length > 0) {
      setErrors(nextErrors);
      showValidationError(firstErrorMessage(nextErrors));
      if (nextErrors.name) {
        document.getElementById(nameId)?.focus();
      } else {
        document.getElementById(emailId)?.focus();
      }
      return;
    }

    setErrors({});
    onClearServerError();
    onSubmit({ name: trimmedName, email: trimmedEmail, is_admin: isAdmin });
  };

  return (
    <Dialog open onOpenChange={open => { if (!open) onClose(); }}>
      <DialogContent
        className="border-border bg-card text-foreground sm:max-w-md"
        onCloseAutoFocus={onCloseAutoFocus}
      >
        <DialogHeader>
          <DialogTitle className="text-foreground">Edit User</DialogTitle>
          <DialogDescription className="text-muted-foreground">
            Update account information for {user.name}.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} noValidate className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor={nameId} className="text-muted-foreground">Name</Label>
            <Input
              id={nameId}
              name="name"
              value={name}
              onChange={e => {
                setName(e.target.value);
                setErrors(current => ({ ...current, name: undefined }));
                onClearServerError();
              }}
              placeholder="Full name"
              maxLength={100}
              required
              aria-required="true"
              aria-invalid={!!errors.name || undefined}
              aria-describedby={describedBy(errors.name && nameErrorId)}
              className={lightInputClassName}
              aria-label="User name"
            />
            {errors.name && (
              <p id={nameErrorId} className="text-xs text-destructive" role="alert">
                {errors.name}
              </p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor={emailId} className="text-muted-foreground">Email</Label>
            <Input
              id={emailId}
              name="email"
              type="email"
              value={email}
              onChange={e => {
                setEmail(e.target.value);
                setErrors(current => ({ ...current, email: undefined }));
                onClearServerError();
              }}
              placeholder="user@example.com"
              required
              aria-required="true"
              aria-invalid={!!errors.email || undefined}
              aria-describedby={describedBy(errors.email && emailErrorId)}
              className={lightInputClassName}
              aria-label="User email"
            />
            {errors.email && (
              <p id={emailErrorId} className="text-xs text-destructive" role="alert">
                {errors.email}
              </p>
            )}
          </div>

          <div className="flex items-center gap-3">
            <input
              id={isAdminId}
              type="checkbox"
              checked={isAdmin}
              onChange={e => setIsAdmin(e.target.checked)}
              className="size-4 rounded-sm border-border bg-muted accent-primary"
              aria-label="Admin privileges"
            />
            <Label htmlFor={isAdminId} className="cursor-pointer text-muted-foreground">
              Admin privileges
            </Label>
          </div>

          {serverError && (
            <p id={formErrorId} className="text-sm text-destructive" role="alert">
              {serverError}
            </p>
          )}

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
  serverError: string;
  onClearServerError: () => void;
  onCloseAutoFocus: DialogCloseAutoFocusHandler;
};

function DeleteUserDialog({
  user,
  onClose,
  onConfirm,
  isPending,
  serverError,
  onClearServerError,
  onCloseAutoFocus,
}: DeleteUserDialogProps) {
  const [confirmText, setConfirmText] = useState("");
  const confirmId = useId();
  const confirmErrorId = `${confirmId}-error`;
  const formErrorId = `${confirmId}-form-error`;
  const invalid = confirmText.length > 0 && confirmText !== "DELETE";

  return (
    <Dialog open onOpenChange={open => { if (!open) onClose(); }}>
      <DialogContent
        className="border-border bg-card text-foreground sm:max-w-md"
        onCloseAutoFocus={onCloseAutoFocus}
      >
        <DialogHeader>
          <DialogTitle className="text-destructive">Delete User</DialogTitle>
          <DialogDescription className="text-muted-foreground">
            This will permanently delete <strong className="text-foreground">{user.name}</strong> ({user.email}).
            This action cannot be undone.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor={confirmId} className="text-muted-foreground">
              Type <span className="font-mono font-bold">DELETE</span> to confirm:
            </Label>
            <Input
              id={confirmId}
              value={confirmText}
              onChange={e => {
                setConfirmText(e.target.value);
                onClearServerError();
              }}
              placeholder="DELETE"
              className={`font-mono ${lightInputClassName}`}
              aria-label="Type DELETE to confirm user deletion"
              aria-invalid={invalid}
              aria-describedby={describedBy(invalid && confirmErrorId)}
            />
            {invalid && (
              <p id={confirmErrorId} className="text-xs text-destructive" role="alert">
                Type DELETE exactly to enable deletion.
              </p>
            )}
          </div>
        </div>

        {serverError && (
          <p id={formErrorId} className="text-sm text-destructive" role="alert">
            {serverError}
          </p>
        )}

        <DialogFooter className="gap-2 sm:gap-0">
          <Button variant="outline" onClick={onClose} disabled={isPending}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            onClick={() => {
              onClearServerError();
              onConfirm();
            }}
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
  serverError: string;
  onClearServerError: () => void;
  onCloseAutoFocus: DialogCloseAutoFocusHandler;
};

function ResetPasswordDialog({
  user,
  onClose,
  onSubmit,
  isPending,
  serverError,
  onClearServerError,
  onCloseAutoFocus,
}: ResetPasswordDialogProps) {
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [errors, setErrors] = useState<UserFormErrors>({});
  const passwordId = useId();
  const confirmPasswordId = useId();
  const passwordDescriptionId = `${passwordId}-description`;
  const passwordErrorId = `${passwordId}-error`;
  const confirmPasswordErrorId = `${confirmPasswordId}-error`;
  const formErrorId = `${confirmPasswordId}-form-error`;

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    const nextErrors: UserFormErrors = {};

    if (password.length < MIN_PASSWORD_LENGTH) {
      nextErrors.password = "Password must be at least 9 characters.";
    } else if (password.length > MAX_PASSWORD_LENGTH) {
      nextErrors.password = "Password must be 128 characters or less.";
    }

    if (password !== confirmPassword) {
      nextErrors.confirmPassword = "Passwords do not match.";
    }

    if (Object.keys(nextErrors).length > 0) {
      setErrors(nextErrors);
      showValidationError(firstErrorMessage(nextErrors));
      if (nextErrors.password) {
        document.getElementById(passwordId)?.focus();
      } else {
        document.getElementById(confirmPasswordId)?.focus();
      }
      return;
    }

    setErrors({});
    onClearServerError();
    onSubmit(password);
  };

  const mismatch = confirmPassword.length > 0 && password !== confirmPassword;
  const passwordError =
    errors.password ??
    (password.length > 0 && password.length < MIN_PASSWORD_LENGTH
      ? "Password must be at least 9 characters."
      : undefined) ??
    (password.length > MAX_PASSWORD_LENGTH
      ? "Password must be 128 characters or less."
      : undefined);
  const confirmPasswordError =
    errors.confirmPassword ?? (mismatch ? "Passwords do not match." : undefined);
  const resetDisabled =
    isPending ||
    password.length < MIN_PASSWORD_LENGTH ||
    password.length > MAX_PASSWORD_LENGTH ||
    mismatch;

  return (
    <Dialog open onOpenChange={open => { if (!open) onClose(); }}>
      <DialogContent
        className="border-border bg-card text-foreground sm:max-w-md"
        onCloseAutoFocus={onCloseAutoFocus}
      >
        <DialogHeader>
          <DialogTitle className="text-foreground">Reset Password</DialogTitle>
          <DialogDescription className="text-muted-foreground">
            Set a new password for <strong className="text-foreground">{user.name}</strong>.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} noValidate className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor={passwordId} className="text-muted-foreground">New Password</Label>
            <Input
              id={passwordId}
              name="password"
              type="password"
              value={password}
              onChange={e => {
                setPassword(e.target.value);
                setErrors(current => ({ ...current, password: undefined }));
                onClearServerError();
              }}
              placeholder="At least 9 characters"
              required
              aria-required="true"
              aria-invalid={!!passwordError || undefined}
              aria-describedby={describedBy(
                passwordDescriptionId,
                passwordError && passwordErrorId,
              )}
              className={lightInputClassName}
              aria-label="New password"
            />
            <p id={passwordDescriptionId} className="text-xs text-muted-foreground">
              Must be 9–128 characters
            </p>
            {passwordError && (
              <p id={passwordErrorId} className="text-xs text-destructive" role="alert">
                {passwordError}
              </p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor={confirmPasswordId} className="text-muted-foreground">Confirm Password</Label>
            <Input
              id={confirmPasswordId}
              name="confirmPassword"
              type="password"
              value={confirmPassword}
              onChange={e => {
                setConfirmPassword(e.target.value);
                setErrors(current => ({
                  ...current,
                  confirmPassword: undefined,
                }));
                onClearServerError();
              }}
              placeholder="Repeat new password"
              required
              aria-required="true"
              className={lightInputClassName}
              aria-label="Confirm new password"
              aria-invalid={!!confirmPasswordError || undefined}
              aria-describedby={describedBy(
                confirmPasswordError && confirmPasswordErrorId,
              )}
            />
            {confirmPasswordError && (
              <p
                id={confirmPasswordErrorId}
                className="text-xs text-destructive"
                role="alert"
              >
                {confirmPasswordError}
              </p>
            )}
          </div>

          {serverError && (
            <p id={formErrorId} className="text-sm text-destructive" role="alert">
              {serverError}
            </p>
          )}

          <DialogFooter className="gap-2 pt-2 sm:gap-0">
            <Button type="button" variant="outline" onClick={onClose} disabled={isPending}>
              Cancel
            </Button>
            <Button
              type="submit"
              variant="accent"
              disabled={resetDisabled}
            >
              {isPending ? "Resetting..." : "Reset Password"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
