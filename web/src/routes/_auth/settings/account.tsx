import { createFileRoute } from "@tanstack/react-router";
import { useQuery, useMutation } from "@tanstack/react-query";
import { useId, useRef, useState, useTransition } from "react";
import type { FormEvent } from "react";
import { Card, CardContent } from "@/components/ui/card";
import { Button, buttonVariants } from "@/components/ui/button";
import { FieldError, FieldHint } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Separator } from "@/components/ui/separator";
import { Avatar, AvatarImage, AvatarFallback } from "@/components/ui/avatar";
import {
  User,
  Lock,
  Image as ImageIcon,
  Upload,
  Trash2,
  AlertTriangle,
} from "lucide-react";
import SettingsCardHeader from "@/components/settings/SettingsCardHeader";
import SettingsErrorCard from "@/components/settings/SettingsErrorCard";
import SettingsLoadingCard from "@/components/settings/SettingsLoadingCard";
import SettingsPage from "@/components/settings/SettingsPage";
import {
  authUserFrom,
  authUserQueryOpts,
  setSignedInUser,
} from "@/lib/query-opts";
import {
  USER_EMAIL_MAX_LENGTH,
  USER_NAME_MAX_LENGTH,
  USER_PASSWORD_MIN_LENGTH,
  MOTION_MICRO_COLORS_CLASS,
  MOTION_SETTINGS_SURFACE_CLASS,
} from "@/lib/constants";
import {
  updateUserName,
  updateUserEmail,
  updateUserPassword,
  updateUserAvatar,
  uploadUserAvatar,
  deleteUserAccount,
} from "@/lib/api";
import { PASSWORD_HINT, passwordRuleError } from "@/lib/form-validation";
import {
  showSuccess,
  showError,
  showActionFailed,
} from "@/lib/toast-helpers";
import { cn, codePointLength, describedBy, getInitials } from "@/lib/utils";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import QuickConnectApproveCard from "@/components/settings/QuickConnectApproveCard";
import DevicesCard from "@/components/settings/DevicesCard";
import ProfilePinCard from "@/components/settings/ProfilePinCard";
import {
  sessionEndedSince,
  sessionStamp,
  useEndSession,
} from "@/hooks/useEndSession";
import { routeHead } from "@/lib/route-head";
import { apiErrorMessage } from "@/lib/is-api-failure";

const ACCOUNT_SETTINGS_HEAD = routeHead("Account Settings");

export const Route = createFileRoute("/_auth/settings/account")({
  head: () => ACCOUNT_SETTINGS_HEAD,
  component: AccountSettings,
});

type AccountErrorField =
  | "name"
  | "email"
  | "avatarUrl"
  | "avatarUpload"
  | "currentPassword"
  | "newPassword"
  | "confirmPassword"
  | "deleteConfirm";

type AccountErrors = Partial<Record<AccountErrorField, string>>;

/** The avatar an optimistic URL change wrote, and the one it replaced. */
type AvatarRollback = { optimistic: string; previous: string | null };

/** Shown when the confirmation field holds anything but the word itself. */
const DELETE_CONFIRM_ERROR = "Type DELETE exactly, in capital letters.";

const MAX_AVATAR_SIZE = 20 * 1024 * 1024;
const ALLOWED_AVATAR_TYPES = [
  "image/jpeg",
  "image/png",
  "image/gif",
  "image/webp",
];

function AccountSettings() {
  const { queryClient } = Route.useRouteContext();
  const endSession = useEndSession();
  const authQuery = authUserQueryOpts();
  const authKey = authQuery.queryKey;
  const { data: userData, isLoading } = useQuery(authQuery);
  const [isDeleting, startDeleteTransition] = useTransition();
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [deleteConfirmText, setDeleteConfirmText] = useState("");
  const deleteAccountButtonRef = useRef<HTMLButtonElement | null>(null);

  const emailId = useId();
  const nameId = useId();
  const avatarUploadId = useId();
  const avatarUrlId = useId();
  const currentPasswordId = useId();
  const newPasswordId = useId();
  const confirmPasswordId = useId();
  const deleteConfirmId = useId();
  const emailErrorId = `${emailId}-error`;
  const nameDescriptionId = `${nameId}-description`;
  const nameErrorId = `${nameId}-error`;
  const avatarUploadDescriptionId = `${avatarUploadId}-description`;
  const avatarUrlDescriptionId = `${avatarUrlId}-description`;
  const avatarUploadErrorId = `${avatarUploadId}-error`;
  const avatarUrlErrorId = `${avatarUrlId}-error`;
  const currentPasswordErrorId = `${currentPasswordId}-error`;
  const newPasswordDescriptionId = `${newPasswordId}-description`;
  const newPasswordErrorId = `${newPasswordId}-error`;
  const confirmPasswordErrorId = `${confirmPasswordId}-error`;
  const deleteConfirmErrorId = `${deleteConfirmId}-error`;

  const user = authUserFrom(userData);

  // Form input state (controlled inputs)
  const [name, setName] = useState<string | null>(null);
  const [email, setEmail] = useState<string | null>(null);
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [avatarUrl, setAvatarUrl] = useState("");
  const [errors, setErrors] = useState<AccountErrors>({});

  const nameValue = name ?? user?.name ?? "";
  const emailValue = email ?? user?.email ?? "";
  const deleteConfirmError =
    deleteConfirmText.length > 0 && deleteConfirmText !== "DELETE"
      ? DELETE_CONFIRM_ERROR
      : errors.deleteConfirm;

  const showFieldError = (
    field: AccountErrorField,
    message: string,
    fieldId: string,
  ) => {
    setErrors(current => ({ ...current, [field]: message }));
    document.getElementById(fieldId)?.focus();
  };

  const clearError = (field: AccountErrorField) => {
    setErrors(current => ({ ...current, [field]: undefined }));
  };

  // Name update mutation
  const updateNameMutation = useMutation({
    mutationFn: (newName: string) => updateUserName(newName),
    onMutate: sessionStamp,
    onError: (err, _newName, stamp) => {
      if (sessionEndedSince(stamp)) return;
      showActionFailed(
        "update name",
        err instanceof Error ? err.message : "An error occurred",
      );
    },
    onSuccess: (res, _newName, stamp) => {
      if (sessionEndedSince(stamp)) return;
      if (res.error) {
        setErrors(current => ({
          ...current,
          name: apiErrorMessage(res, "Failed to update name."),
        }));
        showActionFailed("update name", res);
      } else {
        clearError("name");
        showSuccess("Name updated successfully");
        setSignedInUser(queryClient, res.data.user);
      }
    },
  });

  // Email update mutation
  const updateEmailMutation = useMutation({
    mutationFn: (newEmail: string) => updateUserEmail(newEmail),
    onMutate: sessionStamp,
    onError: (err, _newEmail, stamp) => {
      if (sessionEndedSince(stamp)) return;
      showActionFailed(
        "update email",
        err instanceof Error ? err.message : "An error occurred",
      );
    },
    onSuccess: (res, _newEmail, stamp) => {
      if (sessionEndedSince(stamp)) return;
      if (res.error) {
        setErrors(current => ({
          ...current,
          email: apiErrorMessage(res, "Failed to update email."),
        }));
        showActionFailed("update email", res);
      } else {
        clearError("email");
        showSuccess("Email updated successfully");
        setSignedInUser(queryClient, res.data.user);
      }
    },
  });

  // Password update mutation
  // A password change touches no field held by any cached query.
  // react-doctor-disable-next-line react-doctor/query-mutation-missing-invalidation
  const updatePasswordMutation = useMutation({
    mutationFn: ({
      currentPassword,
      newPassword,
    }: {
      currentPassword: string;
      newPassword: string;
    }) => updateUserPassword(currentPassword, newPassword),
    onSuccess: res => {
      if (res.error) {
        setErrors(current => ({
          ...current,
          currentPassword: apiErrorMessage(res, "Failed to update password."),
        }));
        showActionFailed("update password", res);
      } else {
        setErrors(current => ({
          ...current,
          currentPassword: undefined,
          newPassword: undefined,
          confirmPassword: undefined,
        }));
        showSuccess("Password updated successfully");
        setCurrentPassword("");
        setNewPassword("");
        setConfirmPassword("");
      }
    },
    onError: () => {
      showActionFailed("update password", "An unexpected error occurred");
    },
  });

  // A refused change leaves the server's user as it was, so the cached one
  // stays right too; only the optimistic avatar needs undoing. It is undone
  // on the user cached now, and only while it still shows: a name change or
  // upload that saved meanwhile stays.
  const restoreAvatar = (rollback: AvatarRollback | undefined) => {
    const current = queryClient.getQueryData(authKey);
    if (
      !rollback ||
      current?.error !== false ||
      current.data.user.avatar !== rollback.optimistic
    ) {
      return;
    }
    queryClient.setQueryData(authKey, {
      ...current,
      data: { user: { ...current.data.user, avatar: rollback.previous } },
    });
  };

  // Avatar URL update mutation with optimistic updates
  const updateAvatarUrlMutation = useMutation({
    mutationFn: (avatar: string) => updateUserAvatar(avatar),
    onMutate: async newAvatar => {
      const stamp = sessionStamp();
      await queryClient.cancelQueries({ queryKey: authKey });
      const previousData = queryClient.getQueryData(authKey);
      let rollback: AvatarRollback | undefined;

      if (previousData?.error === false) {
        queryClient.setQueryData(authKey, {
          ...previousData,
          data: {
            user: { ...previousData.data.user, avatar: newAvatar },
          },
        });
        rollback = {
          optimistic: newAvatar,
          previous: previousData.data.user.avatar,
        };
      }

      return { ...stamp, rollback };
    },
    onError: (err, _newAvatar, context) => {
      if (sessionEndedSince(context)) return;
      restoreAvatar(context?.rollback);
      showActionFailed(
        "update avatar",
        err instanceof Error ? err.message : "An error occurred",
      );
    },
    onSuccess: (res, _newAvatar, context) => {
      if (sessionEndedSince(context)) return;
      if (res.error) {
        restoreAvatar(context?.rollback);
        setErrors(current => ({
          ...current,
          avatarUrl: apiErrorMessage(res, "Failed to update avatar."),
        }));
        showActionFailed("update avatar", res);
      } else {
        clearError("avatarUrl");
        showSuccess("Avatar updated successfully");
        setAvatarUrl("");
        // The server's copy replaces the optimistic one: it is what was
        // stored, and the admin user list shows it too.
        setSignedInUser(queryClient, res.data.user);
      }
    },
  });

  // Avatar upload mutation: the server returns the user with the stored
  // avatar, which replaces the cached one.
  const uploadAvatarMutation = useMutation({
    mutationFn: (file: File) => uploadUserAvatar(file),
    onMutate: sessionStamp,
    onError: (err, _file, stamp) => {
      if (sessionEndedSince(stamp)) return;
      showActionFailed(
        "upload avatar",
        err instanceof Error ? err.message : "An error occurred",
      );
    },
    onSuccess: (res, _file, stamp) => {
      if (sessionEndedSince(stamp)) return;
      if (res.error) {
        setErrors(current => ({
          ...current,
          avatarUpload: apiErrorMessage(res, "Failed to upload avatar."),
        }));
        showActionFailed("upload avatar", res);
      } else {
        clearError("avatarUpload");
        showSuccess("Avatar uploaded successfully");
        setSignedInUser(queryClient, res.data.user);
      }
    },
  });

  const handleSaveProfile = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!user) return;

    const trimmedEmail = emailValue.trim();
    const trimmedName = nameValue.trim();

    if (!trimmedEmail) {
      showFieldError("email", "Email is required.", emailId);
      return;
    }
    if (codePointLength(trimmedEmail) > USER_EMAIL_MAX_LENGTH) {
      showFieldError(
        "email",
        `Email must be ${USER_EMAIL_MAX_LENGTH} characters or less.`,
        emailId,
      );
      return;
    }
    if (!trimmedName) {
      showFieldError("name", "Name is required.", nameId);
      return;
    }
    if (codePointLength(trimmedName) > USER_NAME_MAX_LENGTH) {
      showFieldError(
        "name",
        `Name must be ${USER_NAME_MAX_LENGTH} characters or less.`,
        nameId,
      );
      return;
    }

    if (trimmedEmail !== user.email) {
      clearError("email");
      try {
        await updateEmailMutation.mutateAsync(trimmedEmail);
      } catch {
        // The mutation reports its own error; a valid name change can still save.
      }
    }
    if (trimmedName !== user.name) {
      clearError("name");
      updateNameMutation.mutate(trimmedName);
    }
  };

  const handleUpdatePassword = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    const nextErrors: AccountErrors = {};

    if (!currentPassword) {
      nextErrors.currentPassword = "Current password is required.";
    }

    const newPasswordRule = newPassword
      ? passwordRuleError(newPassword, "New password")
      : "New password is required.";
    if (newPasswordRule) {
      nextErrors.newPassword = newPasswordRule;
    }

    if (!confirmPassword) {
      nextErrors.confirmPassword = "Confirm your new password.";
    } else if (newPassword && newPassword !== confirmPassword) {
      nextErrors.confirmPassword = "Passwords do not match.";
    }

    if (nextErrors.currentPassword) {
      showFieldError(
        "currentPassword",
        nextErrors.currentPassword,
        currentPasswordId,
      );
      return;
    }

    if (nextErrors.newPassword) {
      showFieldError("newPassword", nextErrors.newPassword, newPasswordId);
      return;
    }

    if (nextErrors.confirmPassword) {
      showFieldError(
        "confirmPassword",
        nextErrors.confirmPassword,
        confirmPasswordId,
      );
      return;
    }

    setErrors(current => ({
      ...current,
      currentPassword: undefined,
      newPassword: undefined,
      confirmPassword: undefined,
    }));
    updatePasswordMutation.mutate({ currentPassword, newPassword });
  };

  const handleUpdateAvatarUrl = () => {
    if (!avatarUrl.trim()) {
      showFieldError("avatarUrl", "Avatar URL is required.", avatarUrlId);
      return;
    }

    clearError("avatarUrl");
    updateAvatarUrlMutation.mutate(avatarUrl.trim());
  };

  const handleUploadAvatar = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (!ALLOWED_AVATAR_TYPES.includes(file.type)) {
      const message =
        "That file type is not supported. Use JPEG, PNG, GIF or WebP.";
      setErrors(current => ({ ...current, avatarUpload: message }));
      document.getElementById(avatarUploadId)?.focus();
      e.target.value = "";
      return;
    }

    if (file.size > MAX_AVATAR_SIZE) {
      const message = "That file is too large. The limit is 20 MB.";
      setErrors(current => ({ ...current, avatarUpload: message }));
      document.getElementById(avatarUploadId)?.focus();
      e.target.value = "";
      return;
    }

    clearError("avatarUpload");
    uploadAvatarMutation.mutate(file);
    e.target.value = "";
  };

  const handleDeleteAccount = () => {
    if (deleteConfirmText !== "DELETE") {
      showFieldError(
        "deleteConfirm",
        DELETE_CONFIRM_ERROR,
        deleteConfirmId,
      );
      return;
    }

    startDeleteTransition(async () => {
      try {
        const res = await deleteUserAccount();

        if (res.error) {
          showActionFailed("delete account", res);
          setDeleteDialogOpen(false);
          setDeleteConfirmText("");
          return;
        }

        showSuccess("Account deleted successfully");
        await endSession();
      } catch {
        showError("Failed to delete account");
        setDeleteDialogOpen(false);
        setDeleteConfirmText("");
      }
    });
  };

  const handleDeleteDialogOpenChange = (open: boolean) => {
    setDeleteDialogOpen(open);
    if (!open) {
      setDeleteConfirmText("");
      clearError("deleteConfirm");
    }
  };

  const getAvatarUrl = () => user?.avatar || null;

  // Show loading state while fetching user data
  if (isLoading) {
    return <SettingsLoadingCard label="Loading user information..." />;
  }

  // Show error state if user data failed to load
  if (userData?.error || !user) {
    return (
      <SettingsErrorCard
        title="Account unavailable"
        message={
          userData?.error
            ? apiErrorMessage(userData, "Failed to load user information.")
            : "User information not available."
        }
      />
    );
  }

  return (
    <SettingsPage>
      {/* Profile information */}
      <Card className={MOTION_SETTINGS_SURFACE_CLASS}>
        <SettingsCardHeader
          icon={User}
          title="Profile information"
          description="Your name and the email you sign in with."
        />
        <CardContent className="max-w-2xl">
          <form onSubmit={handleSaveProfile} noValidate className="space-y-6">
            {/* Email */}
            <div className="space-y-2">
              <Label htmlFor={emailId} tone="muted">
                Email
              </Label>
              <Input
                id={emailId}
                type="email"
                value={emailValue}
                onChange={e => {
                  setEmail(e.target.value);
                  clearError("email");
                }}
                variant="glass"
                required
                maxLength={USER_EMAIL_MAX_LENGTH}
                aria-required="true"
                aria-invalid={!!errors.email || undefined}
                aria-describedby={describedBy(errors.email && emailErrorId)}
              />
              {errors.email && (
                <FieldError id={emailErrorId}>{errors.email}</FieldError>
              )}
            </div>

            {/* Name */}
            <div className="space-y-2">
              <Label htmlFor={nameId} tone="muted">
                Name
              </Label>
              <FieldHint id={nameDescriptionId}>
                Your display name, up to {USER_NAME_MAX_LENGTH} characters.
              </FieldHint>
              <Input
                id={nameId}
                type="text"
                value={nameValue}
                onChange={e => {
                  setName(e.target.value);
                  clearError("name");
                }}
                variant="glass"
                aria-invalid={!!errors.name || undefined}
                aria-describedby={describedBy(
                  nameDescriptionId,
                  errors.name && nameErrorId,
                )}
              />
              {errors.name && (
                <FieldError id={nameErrorId}>{errors.name}</FieldError>
              )}
            </div>

            <Button
              type="submit"
              disabled={
                updateEmailMutation.isPending ||
                updateNameMutation.isPending ||
                (emailValue.trim() === user.email &&
                  nameValue.trim() === user.name)
              }
              variant="accent"
              className="w-full sm:w-auto"
            >
              {updateEmailMutation.isPending || updateNameMutation.isPending ? (
                <Spinner aria-hidden="true" />
              ) : null}
              {updateEmailMutation.isPending || updateNameMutation.isPending
                ? "Saving..."
                : "Save profile"}
            </Button>
          </form>
        </CardContent>
      </Card>

      {/* Avatar */}
      <Card className={MOTION_SETTINGS_SURFACE_CLASS}>
        <SettingsCardHeader
          icon={ImageIcon}
          title="Avatar"
          description="Update your profile picture."
        />
        <CardContent className="max-w-2xl space-y-6">
          {/* Current Avatar */}
          <div className="flex items-center gap-4">
            {getAvatarUrl() ? (
              <Avatar className="size-20">
                <AvatarImage
                  src={getAvatarUrl() ?? undefined}
                  alt={user.name}
                />
                <AvatarFallback className="bg-primary/20 text-lg text-primary">
                  {getInitials(user.name)}
                </AvatarFallback>
              </Avatar>
            ) : (
              <div className="flex size-20 shrink-0 items-center justify-center rounded-full bg-muted">
                <User className="size-10 text-muted-foreground" aria-hidden="true" />
              </div>
            )}
            <div className="min-w-0">
              <p className="text-sm font-medium text-foreground">{user.name}</p>
              <p className="text-xs break-all text-muted-foreground">{user.email}</p>
            </div>
          </div>

          <Separator className="bg-accent/50" />

          {/* Upload Avatar */}
          <div className="space-y-2">
            {/* Two labels name the file input: this one and the button-like
                one over it, so its name is "Upload image Choose file". */}
            <Label htmlFor={avatarUploadId} tone="muted">
              Upload image
            </Label>
            <FieldHint id={avatarUploadDescriptionId}>
              JPEG, PNG, GIF or WebP, up to 20 MB.
            </FieldHint>
            <div className="flex gap-2">
              <div className="relative flex flex-1">
                <input
                  id={avatarUploadId}
                  type="file"
                  accept={ALLOWED_AVATAR_TYPES.join(",")}
                  onChange={handleUploadAvatar}
                  disabled={uploadAvatarMutation.isPending}
                  className="peer absolute inset-0 z-10 h-9 w-full cursor-pointer opacity-0 disabled:cursor-not-allowed"
                  aria-invalid={!!errors.avatarUpload || undefined}
                  aria-describedby={describedBy(
                    avatarUploadDescriptionId,
                    errors.avatarUpload && avatarUploadErrorId,
                  )}
                />
                <label
                  htmlFor={avatarUploadId}
                  className={cn(
                    buttonVariants({ variant: "outline" }),
                    MOTION_MICRO_COLORS_CLASS,
                    "h-9 flex-1 peer-focus-visible:border-ring peer-focus-visible:ring-[3px] peer-focus-visible:ring-ring/50",
                    uploadAvatarMutation.isPending
                      ? "cursor-not-allowed opacity-70"
                      : "cursor-pointer",
                  )}
                  aria-disabled={uploadAvatarMutation.isPending}
                  aria-busy={uploadAvatarMutation.isPending}
                >
                  <Upload className="size-4" aria-hidden="true" />
                  {uploadAvatarMutation.isPending
                    ? "Uploading..."
                    : "Choose file"}
                </label>
              </div>
            </div>
            {errors.avatarUpload && (
              <FieldError id={avatarUploadErrorId}>
                {errors.avatarUpload}
              </FieldError>
            )}
          </div>

          {/* Set Avatar URL */}
          <div className="space-y-2">
            <Label htmlFor={avatarUrlId} tone="muted">
              Image URL
            </Label>
            <FieldHint id={avatarUrlDescriptionId}>
              Use an image from the web instead of uploading one.
            </FieldHint>
            <div className="flex flex-col gap-2 sm:flex-row">
              <Input
                id={avatarUrlId}
                type="url"
                value={avatarUrl}
                onChange={e => {
                  setAvatarUrl(e.target.value);
                  clearError("avatarUrl");
                }}
                placeholder="https://example.com/avatar.jpg"
                variant="glass"
                className="sm:flex-1"
                aria-invalid={!!errors.avatarUrl || undefined}
                aria-describedby={describedBy(
                  avatarUrlDescriptionId,
                  errors.avatarUrl && avatarUrlErrorId,
                )}
              />
              <Button
                type="button"
                onClick={handleUpdateAvatarUrl}
                disabled={updateAvatarUrlMutation.isPending}
                variant="accent"
                className="w-full sm:w-auto"
              >
                {updateAvatarUrlMutation.isPending ? (
                  <Spinner aria-hidden="true" />
                ) : null}
                {updateAvatarUrlMutation.isPending ? "Saving..." : "Set URL"}
              </Button>
            </div>
            {errors.avatarUrl && (
              <FieldError id={avatarUrlErrorId}>{errors.avatarUrl}</FieldError>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Change password */}
      <Card className={MOTION_SETTINGS_SURFACE_CLASS}>
        <SettingsCardHeader
          icon={Lock}
          title="Change password"
          description="Update the password you sign in with."
        />
        <CardContent className="max-w-2xl">
          {/*
            Standard change-password form (current + new + confirm) with the
            hidden username field below for password managers. Chrome's verbose
            "multiple forms" advisory here is the expected change-password
            heuristic — not actionable.
          */}
          <form
            onSubmit={handleUpdatePassword}
            noValidate
            className="space-y-4"
          >
            <input
              type="email"
              name="username"
              autoComplete="username"
              aria-label="Account email"
              value={emailValue}
              readOnly
              hidden
            />
            <div className="space-y-2">
              <Label htmlFor={currentPasswordId} tone="muted">
                Current password
              </Label>
            <Input
              id={currentPasswordId}
              type="password"
              value={currentPassword}
              onChange={e => {
                setCurrentPassword(e.target.value);
                clearError("currentPassword");
              }}
              variant="glass"
              autoComplete="current-password"
              required
              aria-required="true"
              aria-invalid={!!errors.currentPassword || undefined}
              aria-describedby={describedBy(
                errors.currentPassword && currentPasswordErrorId,
              )}
            />
            {errors.currentPassword && (
              <FieldError id={currentPasswordErrorId}>
                {errors.currentPassword}
              </FieldError>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor={newPasswordId} tone="muted">
              New password
            </Label>
            <FieldHint id={newPasswordDescriptionId}>{PASSWORD_HINT}</FieldHint>
            <Input
              id={newPasswordId}
              type="password"
              value={newPassword}
              onChange={e => {
                setNewPassword(e.target.value);
                clearError("newPassword");
              }}
              variant="glass"
              autoComplete="new-password"
              minLength={USER_PASSWORD_MIN_LENGTH}
              aria-invalid={!!errors.newPassword || undefined}
              aria-describedby={describedBy(
                newPasswordDescriptionId,
                errors.newPassword && newPasswordErrorId,
              )}
            />
            {errors.newPassword && (
              <FieldError id={newPasswordErrorId}>
                {errors.newPassword}
              </FieldError>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor={confirmPasswordId} tone="muted">
              Confirm new password
            </Label>
            <Input
              id={confirmPasswordId}
              type="password"
              value={confirmPassword}
              onChange={e => {
                setConfirmPassword(e.target.value);
                clearError("confirmPassword");
              }}
              variant="glass"
              autoComplete="new-password"
              aria-invalid={!!errors.confirmPassword || undefined}
              aria-describedby={describedBy(
                errors.confirmPassword && confirmPasswordErrorId,
              )}
            />
            {errors.confirmPassword && (
              <FieldError id={confirmPasswordErrorId}>
                {errors.confirmPassword}
              </FieldError>
            )}
          </div>

          <Button
            type="submit"
            disabled={updatePasswordMutation.isPending}
            variant="accent"
            className="w-full sm:w-auto"
          >
            {updatePasswordMutation.isPending ? (
              <Spinner aria-hidden="true" />
            ) : null}
            {updatePasswordMutation.isPending
              ? "Updating..."
              : "Update password"}
          </Button>
          </form>
        </CardContent>
      </Card>

      {/* Profile PIN */}
      <ProfilePinCard />

      {/* Quick Connect */}
      <QuickConnectApproveCard />

      {/* Devices */}
      <DevicesCard />

      {/* Danger Zone — hidden for admins: the server rejects admin
          self-deletion, so there is no action to offer. */}
      {!user.is_admin && (
      <>
      <Card surface="danger" className={MOTION_SETTINGS_SURFACE_CLASS}>
        <SettingsCardHeader
          icon={AlertTriangle}
          title="Danger zone"
          description="Actions here cannot be undone."
          tone="destructive"
        />
        <CardContent className="max-w-2xl space-y-4">
          <div className="space-y-2">
            <p className="text-sm font-medium text-destructive">Delete account</p>
            <p className="text-sm text-destructive">
              Once you delete your account, there is no going back. Please be
              certain.
            </p>
          </div>
          <Button
            ref={deleteAccountButtonRef}
            onClick={() => handleDeleteDialogOpenChange(true)}
            variant="outline-destructive"
            className="w-full sm:w-auto"
          >
            <Trash2 className="size-4" aria-hidden="true" />
            Delete account
          </Button>
        </CardContent>
      </Card>

      {/* Delete Confirmation Dialog */}
      <Dialog open={deleteDialogOpen} onOpenChange={handleDeleteDialogOpenChange}>
        <DialogContent
          onCloseAutoFocus={event => {
            event.preventDefault();
            focusDialogRestoreTarget(deleteAccountButtonRef.current);
          }}
        >
          <DialogHeader>
            <DialogTitle tone="destructive">Delete account</DialogTitle>
            <DialogDescription>
              This action cannot be undone. This will permanently delete your
              account and remove all associated data.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor={deleteConfirmId} tone="muted">
                Type <span className="font-mono font-bold">DELETE</span> to
                confirm
              </Label>
              <Input
                id={deleteConfirmId}
                type="text"
                value={deleteConfirmText}
                onChange={e => {
                  setDeleteConfirmText(e.target.value);
                  clearError("deleteConfirm");
                }}
                variant="glass"
                className="font-mono"
                aria-describedby={describedBy(
                  deleteConfirmError && deleteConfirmErrorId,
                )}
                aria-invalid={!!deleteConfirmError || undefined}
              />
              {deleteConfirmError && (
                <FieldError id={deleteConfirmErrorId}>
                  {deleteConfirmError}
                </FieldError>
              )}
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => handleDeleteDialogOpenChange(false)}
              disabled={isDeleting}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleDeleteAccount}
              disabled={isDeleting || deleteConfirmText !== "DELETE"}
            >
              {isDeleting ? <Spinner aria-hidden="true" /> : null}
              {isDeleting ? "Deleting..." : "Delete account"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
      </>
      )}
    </SettingsPage>
  );
}
