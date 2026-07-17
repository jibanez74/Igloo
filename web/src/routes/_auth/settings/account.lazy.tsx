import { createLazyFileRoute } from "@tanstack/react-router";
import { useQuery, useMutation } from "@tanstack/react-query";
import { useId, useRef, useState, useTransition } from "react";
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
import { authUserQueryOpts } from "@/lib/query-opts";
import {
  AUTH_USER_KEY,
  ADMIN_USERS_KEY,
  USER_EMAIL_MAX_LENGTH,
  USER_NAME_MAX_LENGTH,
  USER_PASSWORD_MAX_LENGTH,
  USER_PASSWORD_MIN_LENGTH,
  MOTION_MICRO_COLORS_CLASS,
} from "@/lib/constants";
import {
  updateUserName,
  updateUserEmail,
  updateUserPassword,
  updateUserAvatar,
  uploadUserAvatar,
  deleteUserAccount,
} from "@/lib/api";
import {
  showSuccess,
  showError,
  showActionFailed,
  showValidationError,
} from "@/lib/toast-helpers";
import { useNavigate } from "@tanstack/react-router";
import type { AuthUser } from "@/types";
import {
  lightInputClassName,
  lightInputPeerHoverClassName,
} from "@/lib/input-styles";
import { cn, codePointLength } from "@/lib/utils";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import QuickConnectApproveCard from "@/components/QuickConnectApproveCard";
import DevicesCard from "@/components/DevicesCard";
import ProfilePinCard from "@/components/ProfilePinCard";

export const Route = createLazyFileRoute("/_auth/settings/account")({
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

const MAX_AVATAR_SIZE = 20 * 1024 * 1024;
const ALLOWED_AVATAR_TYPES = [
  "image/jpeg",
  "image/png",
  "image/gif",
  "image/webp",
  "image/avif",
];

function describedBy(...ids: Array<string | false | null | undefined>) {
  const value = ids.filter(Boolean).join(" ");
  return value || undefined;
}

function AccountSettings() {
  const { queryClient } = Route.useRouteContext();
  const navigate = useNavigate();
  const { data: userData, isLoading } = useQuery(authUserQueryOpts());
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
  const deleteConfirmHelperId = useId();
  const emailErrorId = `${emailId}-error`;
  const nameDescriptionId = `${nameId}-description`;
  const nameErrorId = `${nameId}-error`;
  const avatarUploadDescriptionId = `${avatarUploadId}-description`;
  const avatarUploadErrorId = `${avatarUploadId}-error`;
  const avatarUrlErrorId = `${avatarUrlId}-error`;
  const currentPasswordErrorId = `${currentPasswordId}-error`;
  const newPasswordDescriptionId = `${newPasswordId}-description`;
  const newPasswordErrorId = `${newPasswordId}-error`;
  const confirmPasswordErrorId = `${confirmPasswordId}-error`;
  const deleteConfirmErrorId = `${deleteConfirmId}-error`;

  const user: AuthUser | null =
    userData?.error === false && userData.data?.user
      ? (userData.data.user as AuthUser)
      : null;

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
      ? "Type DELETE exactly to confirm account deletion."
      : errors.deleteConfirm;

  const showFieldError = (
    field: AccountErrorField,
    message: string,
    fieldId: string,
  ) => {
    setErrors(current => ({ ...current, [field]: message }));
    showValidationError(message);
    document.getElementById(fieldId)?.focus();
  };

  const clearError = (field: AccountErrorField) => {
    setErrors(current => ({ ...current, [field]: undefined }));
  };

  // Name update mutation with optimistic updates
  const updateNameMutation = useMutation({
    mutationFn: (newName: string) => updateUserName(newName),
    onMutate: async newName => {
      // Cancel any outgoing refetches
      await queryClient.cancelQueries({ queryKey: [AUTH_USER_KEY] });

      // Snapshot the previous value
      const previousData = queryClient.getQueryData([AUTH_USER_KEY]);

      // Optimistically update the cache
      queryClient.setQueryData<typeof userData>([AUTH_USER_KEY], old => {
        if (!old || old.error || !old.data?.user) return old;
        return {
          ...old,
          data: {
            ...old.data,
            user: {
              ...old.data.user,
              name: newName,
            },
          },
        };
      });

      // Return context with the snapshotted value
      return { previousData };
    },
    onError: (err, _newName, context) => {
      // Rollback to the previous value on error
      if (context?.previousData) {
        queryClient.setQueryData([AUTH_USER_KEY], context.previousData);
      }
      showActionFailed(
        "update name",
        err instanceof Error ? err.message : "An error occurred",
      );
    },
    onSuccess: res => {
      if (res.error) {
        setErrors(current => ({
          ...current,
          name: res.message || "Failed to update name.",
        }));
        showActionFailed("update name", res.message);
        queryClient.invalidateQueries({ queryKey: [AUTH_USER_KEY] });
      } else {
        clearError("name");
        showSuccess("Name updated successfully");
        queryClient.invalidateQueries({ queryKey: [ADMIN_USERS_KEY] });
      }
    },
  });

  // Email update mutation with optimistic updates
  const updateEmailMutation = useMutation({
    mutationFn: (newEmail: string) => updateUserEmail(newEmail),
    onMutate: async newEmail => {
      await queryClient.cancelQueries({ queryKey: [AUTH_USER_KEY] });
      const previousData = queryClient.getQueryData([AUTH_USER_KEY]);

      queryClient.setQueryData<typeof userData>([AUTH_USER_KEY], old => {
        if (!old || old.error || !old.data?.user) return old;
        return {
          ...old,
          data: {
            ...old.data,
            user: { ...old.data.user, email: newEmail },
          },
        };
      });

      return { previousData };
    },
    onError: (err, _newEmail, context) => {
      if (context?.previousData) {
        queryClient.setQueryData([AUTH_USER_KEY], context.previousData);
      }
      showActionFailed(
        "update email",
        err instanceof Error ? err.message : "An error occurred",
      );
    },
    onSuccess: res => {
      if (res.error) {
        setErrors(current => ({
          ...current,
          email: res.message || "Failed to update email.",
        }));
        showActionFailed("update email", res.message);
        queryClient.invalidateQueries({ queryKey: [AUTH_USER_KEY] });
      } else {
        clearError("email");
        showSuccess("Email updated successfully");
        queryClient.invalidateQueries({ queryKey: [ADMIN_USERS_KEY] });
      }
    },
  });

  // Password update mutation
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
          currentPassword: res.message || "Failed to update password.",
        }));
        showActionFailed("update password", res.message);
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

  // Avatar URL update mutation with optimistic updates
  const updateAvatarUrlMutation = useMutation({
    mutationFn: (avatar: string) => updateUserAvatar(avatar),
    onMutate: async newAvatar => {
      await queryClient.cancelQueries({ queryKey: [AUTH_USER_KEY] });
      const previousData = queryClient.getQueryData([AUTH_USER_KEY]);

      queryClient.setQueryData<typeof userData>([AUTH_USER_KEY], old => {
        if (!old || old.error || !old.data?.user) return old;
        return {
          ...old,
          data: {
            ...old.data,
            user: {
              ...old.data.user,
              avatar: newAvatar,
            },
          },
        };
      });

      return { previousData };
    },
    onError: (err, _newAvatar, context) => {
      if (context?.previousData) {
        queryClient.setQueryData([AUTH_USER_KEY], context.previousData);
      }
      showActionFailed(
        "update avatar",
        err instanceof Error ? err.message : "An error occurred",
      );
    },
    onSuccess: res => {
      if (res.error) {
        setErrors(current => ({
          ...current,
          avatarUrl: res.message || "Failed to update avatar.",
        }));
        showActionFailed("update avatar", res.message);
        queryClient.invalidateQueries({ queryKey: [AUTH_USER_KEY] });
      } else {
        clearError("avatarUrl");
        showSuccess("Avatar updated successfully");
        setAvatarUrl("");
        queryClient.invalidateQueries({ queryKey: [ADMIN_USERS_KEY] });
      }
    },
  });

  // Avatar upload mutation with optimistic updates
  const uploadAvatarMutation = useMutation({
    mutationFn: (file: File) => uploadUserAvatar(file),
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey: [AUTH_USER_KEY] });
      const previousData = queryClient.getQueryData([AUTH_USER_KEY]);
      return { previousData };
    },
    onError: (err, _file, context) => {
      if (context?.previousData) {
        queryClient.setQueryData([AUTH_USER_KEY], context.previousData);
      }
      showActionFailed(
        "upload avatar",
        err instanceof Error ? err.message : "An error occurred",
      );
    },
    onSuccess: res => {
      if (res.error) {
        setErrors(current => ({
          ...current,
          avatarUpload: res.message || "Failed to upload avatar.",
        }));
        showActionFailed("upload avatar", res.message);
        queryClient.invalidateQueries({ queryKey: [AUTH_USER_KEY] });
      } else {
        clearError("avatarUpload");
        showSuccess("Avatar uploaded successfully");
        if (res.data?.user) {
          queryClient.setQueryData([AUTH_USER_KEY], {
            error: false,
            data: { user: res.data.user },
          });
        }
        queryClient.invalidateQueries({ queryKey: [ADMIN_USERS_KEY] });
      }
    },
  });

  const handleUpdateName = () => {
    const trimmedName = nameValue.trim();
    if (!user || !trimmedName) {
      showFieldError("name", "Name is required.", nameId);
      return;
    }

    if (trimmedName === user.name) {
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

    clearError("name");
    updateNameMutation.mutate(trimmedName);
  };

  const handleUpdateEmail = () => {
    const trimmedEmail = emailValue.trim();
    if (!user || !trimmedEmail) {
      showFieldError("email", "Email is required.", emailId);
      return;
    }

    if (trimmedEmail === user.email) {
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

    clearError("email");
    updateEmailMutation.mutate(trimmedEmail);
  };

  const handleUpdatePassword = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    const nextErrors: AccountErrors = {};

    if (!currentPassword) {
      nextErrors.currentPassword = "Current password is required.";
    }

    if (!newPassword) {
      nextErrors.newPassword = "New password is required.";
    } else if (codePointLength(newPassword) < USER_PASSWORD_MIN_LENGTH) {
      nextErrors.newPassword = `New password must be at least ${USER_PASSWORD_MIN_LENGTH} characters.`;
    } else if (codePointLength(newPassword) > USER_PASSWORD_MAX_LENGTH) {
      nextErrors.newPassword =
        `New password must be ${USER_PASSWORD_MAX_LENGTH} characters or less.`;
    }

    if (!confirmPassword) {
      nextErrors.confirmPassword = "Confirm your new password.";
    } else if (newPassword && newPassword !== confirmPassword) {
      nextErrors.confirmPassword = "New passwords do not match.";
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
        "Invalid file type. Allowed: JPEG, PNG, GIF, WebP, AVIF.";
      setErrors(current => ({ ...current, avatarUpload: message }));
      showValidationError(message);
      document.getElementById(avatarUploadId)?.focus();
      e.target.value = "";
      return;
    }

    if (file.size > MAX_AVATAR_SIZE) {
      const message = "File too large. Maximum size is 20MB.";
      setErrors(current => ({ ...current, avatarUpload: message }));
      showValidationError(message);
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
        "Type DELETE exactly to confirm account deletion.",
        deleteConfirmId,
      );
      return;
    }

    startDeleteTransition(async () => {
      try {
        const res = await deleteUserAccount();

        if (res.error) {
          showActionFailed("delete account", res.message);
          setDeleteDialogOpen(false);
          setDeleteConfirmText("");
          return;
        }

        showSuccess("Account deleted successfully");
        queryClient.removeQueries();
        await navigate({ to: "/login", replace: true });
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

  const getAvatarUrl = () => {
    if (!user?.avatar) return null;

    // Ensure avatar is a string (handle cases where it might be an object or other type)
    const avatarStr =
      typeof user.avatar === "string" ? user.avatar : String(user.avatar || "");
    if (!avatarStr) return null;

    // If it's a relative path, prepend the API base
    if (avatarStr.startsWith("/api/")) {
      return avatarStr;
    }
    return avatarStr;
  };

  const getInitials = () => {
    if (!user?.name) return "U";
    const parts = user.name.trim().split(/\s+/);
    if (parts.length >= 2) {
      return `${parts[0][0]}${parts[parts.length - 1][0]}`.toUpperCase();
    }
    return user.name[0].toUpperCase();
  };

  // Show loading state while fetching user data
  if (isLoading) {
    return (
      <div className="space-y-8">
        <Card className="border-border/50 bg-muted/30">
          <CardContent className="pt-6">
            <p className="text-muted-foreground">Loading user information...</p>
          </CardContent>
        </Card>
      </div>
    );
  }

  // Show error state if user data failed to load
  if (userData?.error || !user) {
    return (
      <div className="space-y-8">
        <Card className="border-border/50 bg-muted/30">
          <CardContent className="pt-6">
            <p className="text-destructive">
              {userData?.error
                ? userData.message || "Failed to load user information"
                : "User information not available"}
            </p>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      {/* Profile Information */}
      <Card className="border-border/50 bg-muted/30">
        <CardHeader>
          <CardTitle asChild className="flex items-center gap-2 text-foreground">
            <h2>
              <User className="size-5 text-primary" aria-hidden="true" />
              Profile Information
            </h2>
          </CardTitle>
          <CardDescription className="text-muted-foreground">
            Manage your account information and preferences
          </CardDescription>
        </CardHeader>
        <CardContent className="max-w-2xl space-y-6">
          {/* Email */}
          <div className="space-y-2">
            <Label htmlFor={emailId} className="text-muted-foreground">
              Email
            </Label>
            <div className="flex flex-col gap-2 sm:flex-row">
              <Input
                id={emailId}
                type="email"
                value={emailValue}
                onChange={e => {
                  setEmail(e.target.value);
                  clearError("email");
                }}
                placeholder="Enter your email"
                className={`sm:flex-1 ${lightInputClassName}`}
                aria-label="Your email address"
                required
                maxLength={USER_EMAIL_MAX_LENGTH}
                aria-required="true"
                aria-invalid={!!errors.email || undefined}
                aria-describedby={describedBy(errors.email && emailErrorId)}
              />
              <Button
                type="button"
                onClick={handleUpdateEmail}
                disabled={
                  updateEmailMutation.isPending ||
                  emailValue.trim() === user.email
                }
                variant="accent"
                className="w-full sm:w-auto"
              >
                {updateEmailMutation.isPending ? "Saving..." : "Save"}
              </Button>
            </div>
            {errors.email && (
              <p id={emailErrorId} className="text-xs text-destructive" role="alert">
                {errors.email}
              </p>
            )}
          </div>

          {/* Name */}
          <div className="space-y-2">
            <Label htmlFor={nameId} className="text-muted-foreground">
              Name
            </Label>
            <div className="flex flex-col gap-2 sm:flex-row">
              <Input
                id={nameId}
                type="text"
                value={nameValue}
                onChange={e => {
                  setName(e.target.value);
                  clearError("name");
                }}
                placeholder="Enter your name"
                className={`sm:flex-1 ${lightInputClassName}`}
                aria-label="Your display name"
                aria-invalid={!!errors.name || undefined}
                aria-describedby={describedBy(
                  nameDescriptionId,
                  errors.name && nameErrorId,
                )}
              />
              <Button
                type="button"
                onClick={handleUpdateName}
                disabled={
                  updateNameMutation.isPending ||
                  nameValue.trim() === user.name
                }
                variant="accent"
                className="w-full sm:w-auto"
              >
                {updateNameMutation.isPending ? "Saving..." : "Save"}
              </Button>
            </div>
            <p id={nameDescriptionId} className="text-xs text-muted-foreground">
              Your display name (max {USER_NAME_MAX_LENGTH} characters)
            </p>
            {errors.name && (
              <p id={nameErrorId} className="text-xs text-destructive" role="alert">
                {errors.name}
              </p>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Avatar */}
      <Card className="border-border/50 bg-muted/30">
        <CardHeader>
          <CardTitle asChild className="flex items-center gap-2 text-foreground">
            <h2>
              <ImageIcon className="size-5 text-primary" aria-hidden="true" />
              Avatar
            </h2>
          </CardTitle>
          <CardDescription className="text-muted-foreground">
            Update your profile picture
          </CardDescription>
        </CardHeader>
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
                  {getInitials()}
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
            <Label className="text-muted-foreground">Upload Image</Label>
            <div className="flex gap-2">
              <div className="relative flex flex-1">
                <input
                  id={avatarUploadId}
                  type="file"
                  accept="image/jpeg,image/png,image/gif,image/webp,image/avif"
                  onChange={handleUploadAvatar}
                  disabled={uploadAvatarMutation.isPending}
                  className="peer absolute inset-0 z-10 h-9 w-full cursor-pointer opacity-0 disabled:cursor-not-allowed"
                  aria-label="Upload avatar image"
                  aria-invalid={!!errors.avatarUpload || undefined}
                  aria-describedby={describedBy(
                    avatarUploadDescriptionId,
                    errors.avatarUpload && avatarUploadErrorId,
                  )}
                />
                <label
                  htmlFor={avatarUploadId}
                  className={cn(
                    MOTION_MICRO_COLORS_CLASS,
                    "flex h-9 flex-1 items-center justify-center gap-2 rounded-md px-4 py-2 text-sm peer-focus-visible:border-ring/70 peer-focus-visible:ring-[3px] peer-focus-visible:ring-ring/20",
                    lightInputClassName,
                    uploadAvatarMutation.isPending
                      ? "cursor-not-allowed opacity-70"
                      : cn("cursor-pointer", lightInputPeerHoverClassName),
                  )}
                  aria-disabled={uploadAvatarMutation.isPending}
                  aria-busy={uploadAvatarMutation.isPending}
                >
                  <Upload className="size-4" aria-hidden="true" />
                  {uploadAvatarMutation.isPending
                    ? "Uploading..."
                    : "Choose File"}
                </label>
              </div>
            </div>
            <p id={avatarUploadDescriptionId} className="text-xs text-muted-foreground">
              JPEG, PNG, GIF, WebP, or AVIF (max 20MB)
            </p>
            {errors.avatarUpload && (
              <p
                id={avatarUploadErrorId}
                className="text-xs text-destructive"
                role="alert"
              >
                {errors.avatarUpload}
              </p>
            )}
          </div>

          {/* Set Avatar URL */}
          <div className="space-y-2">
            <Label htmlFor={avatarUrlId} className="text-muted-foreground">
              Or enter image URL
            </Label>
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
                className={`sm:flex-1 ${lightInputClassName}`}
                aria-label="Avatar image URL"
                aria-invalid={!!errors.avatarUrl || undefined}
                aria-describedby={describedBy(
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
                {updateAvatarUrlMutation.isPending ? "Saving..." : "Set URL"}
              </Button>
            </div>
            {errors.avatarUrl && (
              <p
                id={avatarUrlErrorId}
                className="text-xs text-destructive"
                role="alert"
              >
                {errors.avatarUrl}
              </p>
            )}
          </div>
        </CardContent>
      </Card>

      {/* Change Password */}
      <Card className="border-border/50 bg-muted/30">
        <CardHeader>
          <CardTitle asChild className="flex items-center gap-2 text-foreground">
            <h2>
              <Lock className="size-5 text-primary" aria-hidden="true" />
              Change Password
            </h2>
          </CardTitle>
          <CardDescription className="text-muted-foreground">
            Update your account password
          </CardDescription>
        </CardHeader>
        <CardContent className="max-w-2xl">
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
              <Label htmlFor={currentPasswordId} className="text-muted-foreground">
                Current Password
              </Label>
            <Input
              id={currentPasswordId}
              type="password"
              value={currentPassword}
              onChange={e => {
                setCurrentPassword(e.target.value);
                clearError("currentPassword");
              }}
              placeholder="Enter current password"
              className={lightInputClassName}
              aria-label="Current password"
              autoComplete="current-password"
              required
              aria-required="true"
              aria-invalid={!!errors.currentPassword || undefined}
              aria-describedby={describedBy(
                errors.currentPassword && currentPasswordErrorId,
              )}
            />
            {errors.currentPassword && (
              <p
                id={currentPasswordErrorId}
                className="text-xs text-destructive"
                role="alert"
              >
                {errors.currentPassword}
              </p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor={newPasswordId} className="text-muted-foreground">
              New Password
            </Label>
            <Input
              id={newPasswordId}
              type="password"
              value={newPassword}
              onChange={e => {
                setNewPassword(e.target.value);
                clearError("newPassword");
              }}
              placeholder="Enter new password"
              className={lightInputClassName}
              aria-label="New password"
              autoComplete="new-password"
              minLength={USER_PASSWORD_MIN_LENGTH}
              aria-invalid={!!errors.newPassword || undefined}
              aria-describedby={describedBy(
                newPasswordDescriptionId,
                errors.newPassword && newPasswordErrorId,
              )}
            />
            <p id={newPasswordDescriptionId} className="text-xs text-muted-foreground">
              Must be at least {USER_PASSWORD_MIN_LENGTH} characters and no more
              than {USER_PASSWORD_MAX_LENGTH} characters
            </p>
            {errors.newPassword && (
              <p
                id={newPasswordErrorId}
                className="text-xs text-destructive"
                role="alert"
              >
                {errors.newPassword}
              </p>
            )}
          </div>

          <div className="space-y-2">
            <Label htmlFor={confirmPasswordId} className="text-muted-foreground">
              Confirm New Password
            </Label>
            <Input
              id={confirmPasswordId}
              type="password"
              value={confirmPassword}
              onChange={e => {
                setConfirmPassword(e.target.value);
                clearError("confirmPassword");
              }}
              placeholder="Confirm new password"
              className={lightInputClassName}
              aria-label="Confirm new password"
              autoComplete="new-password"
              aria-invalid={!!errors.confirmPassword || undefined}
              aria-describedby={describedBy(
                errors.confirmPassword && confirmPasswordErrorId,
              )}
            />
            {errors.confirmPassword && (
              <p
                id={confirmPasswordErrorId}
                className="text-xs text-destructive"
                role="alert"
              >
                {errors.confirmPassword}
              </p>
            )}
          </div>

          <Button
            type="submit"
            disabled={updatePasswordMutation.isPending}
            variant="accent"
            className="w-full sm:w-auto"
          >
            {updatePasswordMutation.isPending
              ? "Updating..."
              : "Update Password"}
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

      {/* Danger Zone */}
      <Card className="border-destructive/50 bg-destructive/10">
        <CardHeader>
          <CardTitle asChild className="flex items-center gap-2 text-destructive">
            <h2>
              <AlertTriangle className="size-5" aria-hidden="true" />
              Danger Zone
            </h2>
          </CardTitle>
          <CardDescription className="text-destructive">
            Irreversible and destructive actions
          </CardDescription>
        </CardHeader>
        <CardContent className="max-w-2xl space-y-4">
          <div className="space-y-2">
            <p className="text-sm font-medium text-destructive">Delete Account</p>
            <p className="text-sm text-destructive">
              Once you delete your account, there is no going back. Please be
              certain.
            </p>
            {user.is_admin && (
              <p className="text-sm font-medium text-primary">
                Note: Admin accounts cannot be deleted.
              </p>
            )}
          </div>
          <Button
            ref={deleteAccountButtonRef}
            onClick={() => handleDeleteDialogOpenChange(true)}
            disabled={user.is_admin}
            variant="destructive"
            className="w-full sm:w-auto"
            aria-label="Delete account"
          >
            <Trash2 className="size-4" aria-hidden="true" />
            Delete Account
          </Button>
        </CardContent>
      </Card>

      {/* Delete Confirmation Dialog */}
      <Dialog open={deleteDialogOpen} onOpenChange={handleDeleteDialogOpenChange}>
        <DialogContent
          className="border-border bg-card text-foreground sm:max-w-md"
          onCloseAutoFocus={event => {
            event.preventDefault();
            focusDialogRestoreTarget(deleteAccountButtonRef.current);
          }}
        >
          <DialogHeader>
            <DialogTitle className="text-destructive">Delete Account</DialogTitle>
            <DialogDescription className="text-muted-foreground">
              This action cannot be undone. This will permanently delete your
              account and remove all associated data.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label
                htmlFor={deleteConfirmId}
                id={deleteConfirmHelperId}
                className="text-muted-foreground"
              >
                Type <span className="font-mono font-bold">DELETE</span> to
                confirm:
              </Label>
              <Input
                id={deleteConfirmId}
                type="text"
                value={deleteConfirmText}
                onChange={e => {
                  setDeleteConfirmText(e.target.value);
                  clearError("deleteConfirm");
                }}
                placeholder="DELETE"
                className={`font-mono ${lightInputClassName}`}
                aria-label="Type DELETE to confirm account deletion"
                aria-describedby={describedBy(
                  deleteConfirmHelperId,
                  deleteConfirmError && deleteConfirmErrorId,
                )}
                aria-invalid={!!deleteConfirmError || undefined}
              />
              {deleteConfirmError && (
                <p
                  id={deleteConfirmErrorId}
                  className="text-xs text-destructive"
                  role="alert"
                >
                  {deleteConfirmError}
                </p>
              )}
            </div>
          </div>
          <DialogFooter className="gap-2 sm:gap-0">
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
              {isDeleting ? "Deleting..." : "Delete Account"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
