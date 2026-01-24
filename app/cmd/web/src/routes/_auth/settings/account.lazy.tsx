import { createLazyFileRoute } from "@tanstack/react-router";
import { useQuery, useQueryClient, useMutation } from "@tanstack/react-query";
import { useState } from "react";
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
  Mail,
  Lock,
  Image as ImageIcon,
  Upload,
  Trash2,
  AlertTriangle,
} from "lucide-react";
import { authUserQueryOpts } from "@/lib/query-opts";
import { AUTH_USER_KEY } from "@/lib/constants";
import {
  updateUserName,
  updateUserPassword,
  updateUserAvatar,
  uploadUserAvatar,
  deleteUserAccount,
} from "@/lib/api";
import { showSuccess, showError, showActionFailed } from "@/lib/toast-helpers";
import { useNavigate } from "@tanstack/react-router";
import { logout } from "@/lib/api";
import type { AuthUser } from "@/types";

export const Route = createLazyFileRoute("/_auth/settings/account")({
  component: AccountSettings,
});

function AccountSettings() {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const { data: userData, isLoading } = useQuery(authUserQueryOpts());
  const [isDeleting, setIsDeleting] = useState(false);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const [deleteConfirmText, setDeleteConfirmText] = useState("");

  const user: AuthUser | null =
    userData?.error === false && userData.data?.user
      ? (userData.data.user as AuthUser)
      : null;

  // Form input state (controlled inputs)
  // Initialize name from user data using lazy initializer
  const [name, setName] = useState(() => user?.name ?? "");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [avatarUrl, setAvatarUrl] = useState("");

  // Sync name when user data loads (only if name is empty, meaning user data loaded after mount)
  // This is a render-time check, but it's safe because setName is idempotent and only runs when needed
  if (user?.name && !name) {
    setName(user.name);
  }

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
        showActionFailed("update name", res.message);
        // Refetch to get correct data
        queryClient.invalidateQueries({ queryKey: [AUTH_USER_KEY] });
      } else {
        showSuccess("Name updated successfully");
        // Name state will automatically reflect the optimistic update
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
        showActionFailed("update password", res.message);
      } else {
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
        showActionFailed("update avatar", res.message);
        queryClient.invalidateQueries({ queryKey: [AUTH_USER_KEY] });
      } else {
        showSuccess("Avatar updated successfully");
        setAvatarUrl("");
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
        showActionFailed("upload avatar", res.message);
        queryClient.invalidateQueries({ queryKey: [AUTH_USER_KEY] });
      } else {
        showSuccess("Avatar uploaded successfully");
        // Update cache with the new user data from response
        if (res.data?.user) {
          queryClient.setQueryData([AUTH_USER_KEY], {
            error: false,
            data: { user: res.data.user },
          });
        }
      }
    },
  });

  const handleUpdateName = () => {
    if (!user || !name.trim()) {
      showError("Name is required");
      return;
    }

    if (name.trim() === user.name) {
      return; // No change
    }

    if (name.length > 100) {
      showError("Name must be 100 characters or less");
      return;
    }

    updateNameMutation.mutate(name.trim());
  };

  const handleUpdatePassword = () => {
    if (!currentPassword || !newPassword) {
      showError("Current and new password are required");
      return;
    }

    if (newPassword.length < 9) {
      showError("New password must be at least 9 characters");
      return;
    }

    if (newPassword.length > 128) {
      showError("New password must be 128 characters or less");
      return;
    }

    if (newPassword !== confirmPassword) {
      showError("New passwords do not match");
      return;
    }

    updatePasswordMutation.mutate({ currentPassword, newPassword });
  };

  const handleUpdateAvatarUrl = () => {
    if (!avatarUrl.trim()) {
      showError("Avatar URL is required");
      return;
    }

    updateAvatarUrlMutation.mutate(avatarUrl.trim());
  };

  const handleUploadAvatar = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    // Validate file type
    const allowedTypes = [
      "image/jpeg",
      "image/png",
      "image/gif",
      "image/webp",
      "image/avif",
    ];
    if (!allowedTypes.includes(file.type)) {
      showError("Invalid file type", "Allowed: JPEG, PNG, GIF, WebP, AVIF");
      e.target.value = "";
      return;
    }

    // Validate file size (20MB)
    const maxSize = 20 * 1024 * 1024;
    if (file.size > maxSize) {
      showError("File too large", "Maximum size is 20MB");
      e.target.value = "";
      return;
    }

    uploadAvatarMutation.mutate(file);
    // Reset file input
    e.target.value = "";
  };

  const handleDeleteAccount = async () => {
    if (deleteConfirmText !== "DELETE") {
      showError("Please type DELETE to confirm");
      return;
    }

    setIsDeleting(true);

    try {
      const res = await deleteUserAccount();

      if (res.error) {
        showActionFailed("delete account", res.message);
        setIsDeleting(false);
        setDeleteDialogOpen(false);
        setDeleteConfirmText("");
      } else {
        showSuccess("Account deleted successfully");
        // Logout and redirect to login
        await logout();
        queryClient.clear();
        navigate({ to: "/login", replace: true });
      }
    } catch {
      showError("Failed to delete account");
      setIsDeleting(false);
      setDeleteDialogOpen(false);
      setDeleteConfirmText("");
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
      <div className='space-y-8'>
        <Card className='border-slate-700/50 bg-slate-800/30'>
          <CardContent className='pt-6'>
            <p className='text-slate-300'>Loading user information...</p>
          </CardContent>
        </Card>
      </div>
    );
  }

  // Show error state if user data failed to load
  if (userData?.error || !user) {
    return (
      <div className='space-y-8'>
        <Card className='border-slate-700/50 bg-slate-800/30'>
          <CardContent className='pt-6'>
            <p className='text-red-400'>
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
    <div className='space-y-8'>
      {/* Profile Information */}
      <Card className='border-slate-700/50 bg-slate-800/30'>
        <CardHeader>
          <CardTitle className='flex items-center gap-2 text-white'>
            <User className='size-5 text-amber-400' aria-hidden='true' />
            Profile Information
          </CardTitle>
          <CardDescription className='text-slate-300'>
            Manage your account information and preferences
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-6'>
          {/* Email (read-only) */}
          <div className='space-y-2'>
            <Label htmlFor='email' className='text-slate-300'>
              Email
            </Label>
            <div className='relative'>
              <Mail className='absolute top-1/2 left-3 size-4 -translate-y-1/2 text-slate-400' />
              <Input
                id='email'
                type='email'
                value={user.email}
                disabled
                className='pl-9 text-slate-400'
                aria-label='Email address (cannot be changed)'
              />
            </div>
            <p className='text-xs text-slate-400'>
              Your email address cannot be changed
            </p>
          </div>

          {/* Name */}
          <div className='space-y-2'>
            <Label htmlFor='name' className='text-slate-300'>
              Name
            </Label>
            <div className='flex gap-2'>
              <Input
                id='name'
                type='text'
                value={name}
                onChange={e => setName(e.target.value)}
                placeholder='Enter your name'
                maxLength={100}
                className='flex-1'
                aria-label='Your display name'
              />
              <Button
                onClick={handleUpdateName}
                disabled={
                  updateNameMutation.isPending ||
                  name.trim() === user.name ||
                  !name.trim()
                }
                variant='accent'
              >
                {updateNameMutation.isPending ? "Saving..." : "Save"}
              </Button>
            </div>
            <p className='text-xs text-slate-400'>
              Your display name (max 100 characters)
            </p>
          </div>
        </CardContent>
      </Card>

      {/* Avatar */}
      <Card className='border-slate-700/50 bg-slate-800/30'>
        <CardHeader>
          <CardTitle className='flex items-center gap-2 text-white'>
            <ImageIcon className='size-5 text-amber-400' aria-hidden='true' />
            Avatar
          </CardTitle>
          <CardDescription className='text-slate-300'>
            Update your profile picture
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-6'>
          {/* Current Avatar */}
          <div className='flex items-center gap-4'>
            {getAvatarUrl() ? (
              <Avatar className='size-20'>
                <AvatarImage
                  src={getAvatarUrl() ?? undefined}
                  alt={user.name}
                />
                <AvatarFallback className='bg-amber-500/20 text-lg text-amber-400'>
                  {getInitials()}
                </AvatarFallback>
              </Avatar>
            ) : (
              <div className='flex size-20 shrink-0 items-center justify-center rounded-full bg-slate-800'>
                <User className='size-10 text-slate-600' aria-hidden='true' />
              </div>
            )}
            <div>
              <p className='text-sm font-medium text-white'>{user.name}</p>
              <p className='text-xs text-slate-400'>{user.email}</p>
            </div>
          </div>

          <Separator className='bg-slate-700/50' />

          {/* Upload Avatar */}
          <div className='space-y-2'>
            <Label className='text-slate-300'>Upload Image</Label>
            <div className='flex gap-2'>
              <label
                htmlFor='avatar-upload'
                className='flex flex-1 cursor-pointer items-center justify-center gap-2 rounded-md border border-slate-600 bg-slate-800/50 px-4 py-2 text-sm text-slate-300 transition-colors hover:bg-slate-700/50'
              >
                <Upload className='size-4' aria-hidden='true' />
                {uploadAvatarMutation.isPending
                  ? "Uploading..."
                  : "Choose File"}
              </label>
              <input
                id='avatar-upload'
                type='file'
                accept='image/jpeg,image/png,image/gif,image/webp,image/avif'
                onChange={handleUploadAvatar}
                disabled={uploadAvatarMutation.isPending}
                className='sr-only'
                aria-label='Upload avatar image'
              />
            </div>
            <p className='text-xs text-slate-400'>
              JPEG, PNG, GIF, WebP, or AVIF (max 20MB)
            </p>
          </div>

          {/* Set Avatar URL */}
          <div className='space-y-2'>
            <Label htmlFor='avatar-url' className='text-slate-300'>
              Or enter image URL
            </Label>
            <div className='flex gap-2'>
              <Input
                id='avatar-url'
                type='url'
                value={avatarUrl}
                onChange={e => setAvatarUrl(e.target.value)}
                placeholder='https://example.com/avatar.jpg'
                className='flex-1'
                aria-label='Avatar image URL'
              />
              <Button
                onClick={handleUpdateAvatarUrl}
                disabled={
                  updateAvatarUrlMutation.isPending || !avatarUrl.trim()
                }
                variant='accent'
              >
                {updateAvatarUrlMutation.isPending ? "Saving..." : "Set URL"}
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Change Password */}
      <Card className='border-slate-700/50 bg-slate-800/30'>
        <CardHeader>
          <CardTitle className='flex items-center gap-2 text-white'>
            <Lock className='size-5 text-amber-400' aria-hidden='true' />
            Change Password
          </CardTitle>
          <CardDescription className='text-slate-300'>
            Update your account password
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='space-y-2'>
            <Label htmlFor='current-password' className='text-slate-300'>
              Current Password
            </Label>
            <Input
              id='current-password'
              type='password'
              value={currentPassword}
              onChange={e => setCurrentPassword(e.target.value)}
              placeholder='Enter current password'
              aria-label='Current password'
            />
          </div>

          <div className='space-y-2'>
            <Label htmlFor='new-password' className='text-slate-300'>
              New Password
            </Label>
            <Input
              id='new-password'
              type='password'
              value={newPassword}
              onChange={e => setNewPassword(e.target.value)}
              placeholder='Enter new password'
              aria-label='New password'
            />
            <p className='text-xs text-slate-400'>
              Must be at least 9 characters and no more than 128 characters
            </p>
          </div>

          <div className='space-y-2'>
            <Label htmlFor='confirm-password' className='text-slate-300'>
              Confirm New Password
            </Label>
            <Input
              id='confirm-password'
              type='password'
              value={confirmPassword}
              onChange={e => setConfirmPassword(e.target.value)}
              placeholder='Confirm new password'
              aria-label='Confirm new password'
            />
          </div>

          <Button
            onClick={handleUpdatePassword}
            disabled={
              updatePasswordMutation.isPending ||
              !currentPassword ||
              !newPassword ||
              !confirmPassword
            }
            variant='accent'
            className='w-full sm:w-auto'
          >
            {updatePasswordMutation.isPending
              ? "Updating..."
              : "Update Password"}
          </Button>
        </CardContent>
      </Card>

      {/* Danger Zone */}
      <Card className='border-red-500/50 bg-red-950/20'>
        <CardHeader>
          <CardTitle className='flex items-center gap-2 text-red-400'>
            <AlertTriangle className='size-5' aria-hidden='true' />
            Danger Zone
          </CardTitle>
          <CardDescription className='text-red-300/80'>
            Irreversible and destructive actions
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='space-y-2'>
            <p className='text-sm font-medium text-red-400'>Delete Account</p>
            <p className='text-sm text-red-300/80'>
              Once you delete your account, there is no going back. Please be
              certain.
            </p>
            {user.is_admin && (
              <p className='text-sm font-medium text-amber-400'>
                Note: Admin accounts cannot be deleted.
              </p>
            )}
          </div>
          <Button
            onClick={() => setDeleteDialogOpen(true)}
            disabled={user.is_admin}
            variant='destructive'
            className='w-full sm:w-auto'
            aria-label='Delete account'
          >
            <Trash2 className='size-4' aria-hidden='true' />
            Delete Account
          </Button>
        </CardContent>
      </Card>

      {/* Delete Confirmation Dialog */}
      <Dialog open={deleteDialogOpen} onOpenChange={setDeleteDialogOpen}>
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle className='text-red-400'>Delete Account</DialogTitle>
            <DialogDescription className='text-slate-300'>
              This action cannot be undone. This will permanently delete your
              account and remove all associated data.
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-4 py-4'>
            <div className='space-y-2'>
              <Label htmlFor='delete-confirm' className='text-slate-300'>
                Type <span className='font-mono font-bold'>DELETE</span> to
                confirm:
              </Label>
              <Input
                id='delete-confirm'
                type='text'
                value={deleteConfirmText}
                onChange={e => setDeleteConfirmText(e.target.value)}
                placeholder='DELETE'
                className='font-mono'
                aria-label='Type DELETE to confirm account deletion'
              />
            </div>
          </div>
          <DialogFooter className='gap-2 sm:gap-0'>
            <Button
              variant='outline'
              onClick={() => {
                setDeleteDialogOpen(false);
                setDeleteConfirmText("");
              }}
              disabled={isDeleting}
            >
              Cancel
            </Button>
            <Button
              variant='destructive'
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
