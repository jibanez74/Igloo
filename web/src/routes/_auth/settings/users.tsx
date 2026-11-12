import { createFileRoute, Link } from "@tanstack/react-router";
import { useQuery, useQueryClient, useMutation } from "@tanstack/react-query";
import { useId, useRef, useState } from "react";
import type { FormEvent } from "react";
import { Alert } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Button, buttonVariants } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { FieldError, FieldHint } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import SettingsCardHeader from "@/components/settings/SettingsCardHeader";
import SettingsPage from "@/components/settings/SettingsPage";
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
import {
  adminUsersQueryOpts,
  authUserQueryOpts,
  watchRoomInviteUsersQueryOpts,
} from "@/lib/query-opts";
import {
  USER_EMAIL_MAX_LENGTH,
  MOTION_SETTINGS_SURFACE_CLASS,
  USER_NAME_MAX_LENGTH,
} from "@/lib/constants";
import {
  adminCreateUser,
  adminUpdateUser,
  adminDeleteUser,
  adminResetUserPassword,
} from "@/lib/api";
import {
  isValidEmail,
  PASSWORD_HINT,
  passwordRuleError,
} from "@/lib/form-validation";
import { cn, codePointLength, describedBy, getInitials } from "@/lib/utils";
import { showSuccess, showActionFailed } from "@/lib/toast-helpers";
import type { AdminUserType } from "@/types";
import { useDialogFocusRestore } from "@/hooks/useDialogFocusRestore";
import { routeHead } from "@/lib/route-head";
import { apiErrorMessage } from "@/lib/is-api-failure";

const USERS_SETTINGS_HEAD = routeHead("User Management");
const USER_ACTION_FALLBACK_MESSAGE = "Something went wrong. Please try again.";

export const Route = createFileRoute("/_auth/settings/users")({
  head: () => USERS_SETTINGS_HEAD,
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

// Shared name/email validation for the create and edit user dialogs. Inputs are
// already trimmed; `ignoredUserId` excludes the edited user from the duplicate
// check. Returns only the fields that failed.
function validateNameEmail(
  users: AdminUserType[],
  name: string,
  email: string,
  ignoredUserId?: number,
): UserFormErrors {
  const errors: UserFormErrors = {};

  if (name === "") {
    errors.name = "Name is required.";
  } else if (codePointLength(name) > USER_NAME_MAX_LENGTH) {
    errors.name = `Name must be ${USER_NAME_MAX_LENGTH} characters or less.`;
  }

  if (email === "") {
    errors.email = "Email is required.";
  } else if (!isValidEmail(email)) {
    errors.email = "Enter a valid email address.";
  } else if (codePointLength(email) > USER_EMAIL_MAX_LENGTH) {
    errors.email = `Email must be ${USER_EMAIL_MAX_LENGTH} characters or less.`;
  } else if (hasDuplicateEmail(users, email, ignoredUserId)) {
    errors.email = "A user with that email already exists.";
  }

  return errors;
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

  // Both the admin list and the watch-room invite list show every user.
  const refreshUserLists = () => {
    void queryClient.invalidateQueries({
      queryKey: adminUsersQueryOpts().queryKey,
    });
    void queryClient.invalidateQueries({
      queryKey: watchRoomInviteUsersQueryOpts().queryKey,
    });
  };

  const createMutation = useMutation({
    mutationFn: adminCreateUser,
    onSuccess: res => {
      if (res.error) {
        setDialogError(apiErrorMessage(res, USER_ACTION_FALLBACK_MESSAGE));
        showActionFailed("create user", res);
        return;
      }
      showSuccess("User created successfully");
      refreshUserLists();
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
        setDialogError(apiErrorMessage(res, USER_ACTION_FALLBACK_MESSAGE));
        showActionFailed("update user", res);
        return;
      }
      showSuccess("User updated successfully");
      refreshUserLists();
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
        setDialogError(apiErrorMessage(res, USER_ACTION_FALLBACK_MESSAGE));
        showActionFailed("delete user", res);
        return;
      }
      showSuccess("User deleted successfully");
      refreshUserLists();
      closeDialog();
    },
    onError: () => {
      const message = "An unexpected error occurred";
      setDialogError(message);
      showActionFailed("delete user", message);
    },
  });

  // An admin password reset changes nothing either user list renders,
  // which is why it is the only mutation here without an invalidate.
  // react-doctor-disable-next-line react-doctor/query-mutation-missing-invalidation
  const resetPasswordMutation = useMutation({
    mutationFn: ({ id, password }: { id: number; password: string }) =>
      adminResetUserPassword(id, password),
    onSuccess: res => {
      if (res.error) {
        setDialogError(apiErrorMessage(res, USER_ACTION_FALLBACK_MESSAGE));
        showActionFailed("reset password", res);
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
    <SettingsPage>
      <Card className={MOTION_SETTINGS_SURFACE_CLASS}>
        <SettingsCardHeader
          icon={Users}
          title="User management"
          description="Create, edit, and remove user accounts."
          action={
            <Button
              ref={addUserButtonRef}
              variant="accent"
              onClick={event =>
                openDialog({ type: "create" }, event.currentTarget)
              }
              className="w-full shrink-0 sm:w-auto"
            >
              <UserPlus className="size-4" aria-hidden="true" />
              Add user
            </Button>
          }
        />

        <CardContent>
          {isLoading && (
            <p
              role="status"
              className="flex items-center gap-3 text-sm text-muted-foreground"
            >
              <Spinner tone="primary" aria-hidden="true" />
              Loading users...
            </p>
          )}

          {!isLoading && usersData?.error && (
            <Alert>
              {apiErrorMessage(usersData, "Couldn’t load users. Please try again.")}
            </Alert>
          )}

          {!isLoading && !usersData?.error && users.length === 0 && (
            <p className="text-sm text-muted-foreground">No users found.</p>
          )}

          {!isLoading && users.length > 0 && (
            <ul className="divide-y divide-border/50" role="list" aria-label="User list">
              {users.map(user => (
                <li key={user.id} className="flex flex-col gap-3 py-4 sm:flex-row sm:items-center sm:gap-4">
                  <div className="flex min-w-0 items-center gap-3 sm:flex-1">
                    <Avatar className="size-10 shrink-0">
                      {user.avatar && (
                        <AvatarImage src={user.avatar} alt={user.name} />
                      )}
                      <AvatarFallback className="bg-primary/10 text-primary">
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
                        <Badge variant="tinted" size="sm">
                          <ShieldCheck className="size-3" aria-hidden="true" />
                          Admin
                        </Badge>
                      ) : (
                        <Badge variant="outline" size="sm">
                          <ShieldOff className="size-3" aria-hidden="true" />
                          User
                        </Badge>
                      )}
                    </div>

                    <div className="flex shrink-0 items-center gap-2">
                      {authResolved && user.id === currentUserId ? (
                        <Link
                          to="/settings/account"
                          className={cn(
                            buttonVariants({ variant: "ghost", size: "sm" }),
                            "text-xs text-muted-foreground hover:text-primary",
                          )}
                        >
                          Account settings
                        </Link>
                      ) : (
                        <>
                          <Button
                            variant="ghost-muted"
                            size="icon"
                            onClick={event =>
                              openDialog(
                                { type: "reset-password", user },
                                event.currentTarget,
                              )
                            }
                            aria-label={`Reset password for ${user.name}`}
                          >
                            <KeyRound className="size-4" aria-hidden="true" />
                          </Button>
                          <Button
                            variant="ghost-muted"
                            size="icon"
                            onClick={event =>
                              openDialog({ type: "edit", user }, event.currentTarget)
                            }
                            aria-label={`Edit ${user.name}`}
                          >
                            <Pencil className="size-4" aria-hidden="true" />
                          </Button>
                          <Button
                            variant="ghost-destructive"
                            size="icon"
                            onClick={event =>
                              openDialog({ type: "delete", user }, event.currentTarget)
                            }
                            aria-label={`Delete ${user.name}`}
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
    </SettingsPage>
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

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    const trimmedName = name.trim();
    const trimmedEmail = email.trim();
    const nextErrors = validateNameEmail(users, trimmedName, trimmedEmail);

    const passwordRule = passwordRuleError(password);
    if (passwordRule) {
      nextErrors.password = passwordRule;
    }

    if (Object.keys(nextErrors).length > 0) {
      setErrors(nextErrors);
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
        onCloseAutoFocus={onCloseAutoFocus}
      >
        <DialogHeader>
          <DialogTitle>Add user</DialogTitle>
          <DialogDescription>
            Create a new user account.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} noValidate className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor={nameId} tone="muted">Name</Label>
            <Input
              id={nameId}
              name="name"
              value={name}
              onChange={e => {
                setName(e.target.value);
                setErrors(current => ({ ...current, name: undefined }));
                onClearServerError();
              }}
              required
              aria-required="true"
              aria-invalid={!!errors.name || undefined}
              aria-describedby={describedBy(errors.name && nameErrorId)}
              variant="glass"
            />
            {errors.name && (
              <FieldError id={nameErrorId}>{errors.name}</FieldError>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor={emailId} tone="muted">Email</Label>
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
              maxLength={USER_EMAIL_MAX_LENGTH}
              required
              aria-required="true"
              aria-invalid={!!errors.email || undefined}
              aria-describedby={describedBy(errors.email && emailErrorId)}
              variant="glass"
            />
            {errors.email && (
              <FieldError id={emailErrorId}>{errors.email}</FieldError>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor={passwordId} tone="muted">Password</Label>
            <FieldHint id={passwordDescriptionId}>{PASSWORD_HINT}</FieldHint>
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
              required
              aria-required="true"
              aria-invalid={!!errors.password || undefined}
              aria-describedby={describedBy(
                passwordDescriptionId,
                errors.password && passwordErrorId,
              )}
              variant="glass"
            />
            {errors.password && (
              <FieldError id={passwordErrorId}>{errors.password}</FieldError>
            )}
          </div>

          <div className="flex items-center gap-3">
            <Checkbox
              id={isAdminId}
              checked={isAdmin}
              onCheckedChange={checked => setIsAdmin(checked === true)}
            />
            <Label
              htmlFor={isAdminId}
              tone="muted"
              className="mb-0 cursor-pointer"
            >
              Grant admin privileges
            </Label>
          </div>

          {serverError && (
            <FieldError>{serverError}</FieldError>
          )}

          <DialogFooter className="pt-2">
            <Button type="button" variant="outline" onClick={onClose} disabled={isPending}>
              Cancel
            </Button>
            <Button type="submit" variant="accent" disabled={isPending}>
              {isPending ? <Spinner aria-hidden="true" /> : null}
              {isPending ? "Creating..." : "Create user"}
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

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    const trimmedName = name.trim();
    const trimmedEmail = email.trim();
    const nextErrors = validateNameEmail(users, trimmedName, trimmedEmail, user.id);

    if (Object.keys(nextErrors).length > 0) {
      setErrors(nextErrors);
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
        onCloseAutoFocus={onCloseAutoFocus}
      >
        <DialogHeader>
          <DialogTitle>Edit user</DialogTitle>
          <DialogDescription>
            Update account information for {user.name}.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} noValidate className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor={nameId} tone="muted">Name</Label>
            <Input
              id={nameId}
              name="name"
              value={name}
              onChange={e => {
                setName(e.target.value);
                setErrors(current => ({ ...current, name: undefined }));
                onClearServerError();
              }}
              required
              aria-required="true"
              aria-invalid={!!errors.name || undefined}
              aria-describedby={describedBy(errors.name && nameErrorId)}
              variant="glass"
            />
            {errors.name && (
              <FieldError id={nameErrorId}>{errors.name}</FieldError>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor={emailId} tone="muted">Email</Label>
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
              maxLength={USER_EMAIL_MAX_LENGTH}
              required
              aria-required="true"
              aria-invalid={!!errors.email || undefined}
              aria-describedby={describedBy(errors.email && emailErrorId)}
              variant="glass"
            />
            {errors.email && (
              <FieldError id={emailErrorId}>{errors.email}</FieldError>
            )}
          </div>

          <div className="flex items-center gap-3">
            <Checkbox
              id={isAdminId}
              checked={isAdmin}
              onCheckedChange={checked => setIsAdmin(checked === true)}
            />
            <Label
              htmlFor={isAdminId}
              tone="muted"
              className="mb-0 cursor-pointer"
            >
              Admin privileges
            </Label>
          </div>

          {serverError && (
            <FieldError>{serverError}</FieldError>
          )}

          <DialogFooter className="pt-2">
            <Button type="button" variant="outline" onClick={onClose} disabled={isPending}>
              Cancel
            </Button>
            <Button type="submit" variant="accent" disabled={isPending}>
              {isPending ? <Spinner aria-hidden="true" /> : null}
              {isPending ? "Saving..." : "Save changes"}
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
  const invalid = confirmText.length > 0 && confirmText !== "DELETE";

  return (
    <Dialog open onOpenChange={open => { if (!open) onClose(); }}>
      <DialogContent
        onCloseAutoFocus={onCloseAutoFocus}
      >
        <DialogHeader>
          <DialogTitle tone="destructive">Delete user</DialogTitle>
          <DialogDescription>
            This will permanently delete <strong className="text-foreground">{user.name}</strong> ({user.email}).
            This action cannot be undone.
          </DialogDescription>
        </DialogHeader>

        {/* A form, so Enter in its one field confirms once DELETE is typed;
            the submit button is disabled until then. */}
        <form
          noValidate
          className="space-y-4 py-2"
          onSubmit={event => {
            event.preventDefault();
            if (confirmText !== "DELETE") return;
            onClearServerError();
            onConfirm();
          }}
        >
          <div className="space-y-2">
            <Label htmlFor={confirmId} tone="muted">
              Type <span className="font-mono font-bold">DELETE</span> to confirm
            </Label>
            <Input
              id={confirmId}
              value={confirmText}
              onChange={e => {
                setConfirmText(e.target.value);
                onClearServerError();
              }}
              variant="glass"
              className="font-mono"
              aria-invalid={invalid || undefined}
              aria-describedby={describedBy(invalid && confirmErrorId)}
            />
            {invalid && (
              <FieldError id={confirmErrorId}>
                Type DELETE exactly, in capital letters.
              </FieldError>
            )}
          </div>

        {serverError && <FieldError>{serverError}</FieldError>}

        <DialogFooter className="pt-2">
          <Button
            type="button"
            variant="outline"
            onClick={onClose}
            disabled={isPending}
          >
            Cancel
          </Button>
          <Button
            type="submit"
            variant="destructive"
            disabled={isPending || confirmText !== "DELETE"}
          >
            {isPending ? <Spinner aria-hidden="true" /> : null}
            {isPending ? "Deleting..." : "Delete user"}
          </Button>
        </DialogFooter>
        </form>
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

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    const nextErrors: UserFormErrors = {};

    const passwordRule = passwordRuleError(password);
    if (passwordRule) {
      nextErrors.password = passwordRule;
    }

    if (password !== confirmPassword) {
      nextErrors.confirmPassword = "Passwords do not match.";
    }

    if (Object.keys(nextErrors).length > 0) {
      setErrors(nextErrors);
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
  const passwordRule = passwordRuleError(password);
  const passwordError =
    errors.password ?? (password.length > 0 ? passwordRule : null) ?? undefined;
  const confirmPasswordError =
    errors.confirmPassword ?? (mismatch ? "Passwords do not match." : undefined);
  const resetDisabled =
    isPending ||
    passwordRule !== null ||
    mismatch;

  return (
    <Dialog open onOpenChange={open => { if (!open) onClose(); }}>
      <DialogContent
        onCloseAutoFocus={onCloseAutoFocus}
      >
        <DialogHeader>
          <DialogTitle>Reset password</DialogTitle>
          <DialogDescription>
            Set a new password for <strong className="text-foreground">{user.name}</strong>.
          </DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} noValidate className="space-y-4 py-2">
          <div className="space-y-2">
            <Label htmlFor={passwordId} tone="muted">New password</Label>
            <FieldHint id={passwordDescriptionId}>{PASSWORD_HINT}</FieldHint>
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
              required
              aria-required="true"
              aria-invalid={!!passwordError || undefined}
              aria-describedby={describedBy(
                passwordDescriptionId,
                passwordError && passwordErrorId,
              )}
              variant="glass"
            />
            {passwordError && (
              <FieldError id={passwordErrorId}>{passwordError}</FieldError>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor={confirmPasswordId} tone="muted">
              Confirm new password
            </Label>
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
              required
              aria-required="true"
              variant="glass"
              aria-invalid={!!confirmPasswordError || undefined}
              aria-describedby={describedBy(
                confirmPasswordError && confirmPasswordErrorId,
              )}
            />
            {confirmPasswordError && (
              <FieldError id={confirmPasswordErrorId}>
                {confirmPasswordError}
              </FieldError>
            )}
          </div>

          {serverError && (
            <FieldError>{serverError}</FieldError>
          )}

          <DialogFooter className="pt-2">
            <Button type="button" variant="outline" onClick={onClose} disabled={isPending}>
              Cancel
            </Button>
            <Button
              type="submit"
              variant="accent"
              disabled={resetDisabled}
            >
              {isPending ? <Spinner aria-hidden="true" /> : null}
              {isPending ? "Resetting..." : "Reset password"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
