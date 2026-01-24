import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  showCreated,
  showUpdated,
  showActionFailed,
  showValidationError,
} from "@/lib/toast-helpers";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { createPlaylist, updatePlaylist } from "@/lib/api";
import { PLAYLISTS_KEY, PLAYLIST_DETAILS_KEY } from "@/lib/constants";
import { unwrapString } from "@/lib/nullable";
import type { NullableString } from "@/types";

// ============================================================================
// Types
// ============================================================================

type PlaylistData = {
  id: number;
  name: string;
  description: NullableString;
  is_public: boolean;
};

type CreateModeProps = {
  mode: "create";
  playlist?: never;
};

type EditModeProps = {
  mode: "edit";
  playlist: PlaylistData;
};

type PlaylistFormDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
} & (CreateModeProps | EditModeProps);

// ============================================================================
// Dialog Configuration
// ============================================================================

const DIALOG_CONFIG = {
  create: {
    title: "Create New Playlist",
    description: "Create a new playlist to organize your favorite tracks.",
    submitText: "Create Playlist",
    pendingText: "Creating...",
    successMessage: "Playlist created",
    errorMessage: "Failed to create playlist",
  },
  edit: {
    title: "Edit Playlist",
    description: "Update the playlist details.",
    submitText: "Save Changes",
    pendingText: "Saving...",
    successMessage: "Playlist updated",
    errorMessage: "Failed to update playlist",
  },
} as const;

// ============================================================================
// Main Component
// ============================================================================

export default function PlaylistFormDialog(props: PlaylistFormDialogProps) {
  const { open, onOpenChange, mode } = props;

  // For edit mode, we use a key to force remount when playlist changes
  const formKey = mode === "edit" ? props.playlist.id : "create";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      {open && (
        <PlaylistForm
          key={formKey}
          mode={mode}
          playlist={mode === "edit" ? props.playlist : undefined}
          onOpenChange={onOpenChange}
        />
      )}
    </Dialog>
  );
}

// ============================================================================
// Form Component
// ============================================================================

type PlaylistFormProps = {
  mode: "create" | "edit";
  playlist?: PlaylistData;
  onOpenChange: (open: boolean) => void;
};

function PlaylistForm({ mode, playlist, onOpenChange }: PlaylistFormProps) {
  const config = DIALOG_CONFIG[mode];
  const queryClient = useQueryClient();

  // Initialize form state based on mode
  const [name, setName] = useState(playlist?.name ?? "");
  const [description, setDescription] = useState(
    unwrapString(playlist?.description) ?? ""
  );

  // Create mutation
  const createMutation = useMutation({
    mutationFn: () =>
      createPlaylist({
        name: name.trim(),
        description: description.trim() || undefined,
        is_public: false,
      }),
    onSuccess: (data) => {
      if (data.error) {
        showActionFailed("create playlist", data.message);
        return;
      }
      queryClient.invalidateQueries({ queryKey: [PLAYLISTS_KEY] });
      showCreated("Playlist", `"${name}" has been created successfully.`);
      handleClose();
    },
    onError: () => {
      showActionFailed("create playlist", "An unexpected error occurred. Please try again.");
    },
  });

  // Update mutation
  const updateMutation = useMutation({
    mutationFn: () => {
      if (!playlist) throw new Error("Playlist is required for edit mode");
      return updatePlaylist(playlist.id, {
        name: name.trim(),
        description: description.trim() || undefined,
        is_public: playlist.is_public,
      });
    },
    onSuccess: (data) => {
      if (data.error) {
        showActionFailed("update playlist", data.message);
        return;
      }
      queryClient.invalidateQueries({ queryKey: [PLAYLISTS_KEY] });
      if (playlist) {
        queryClient.invalidateQueries({
          queryKey: [PLAYLIST_DETAILS_KEY, playlist.id],
        });
      }
      showUpdated("Playlist");
      onOpenChange(false);
    },
    onError: () => {
      showActionFailed("update playlist");
    },
  });

  // Use the appropriate mutation based on mode
  const mutation = mode === "create" ? createMutation : updateMutation;

  const handleClose = () => {
    setName("");
    setDescription("");
    onOpenChange(false);
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    if (!name.trim()) {
      showValidationError("Playlist name is required");
      return;
    }

    if (name.trim().length > 255) {
      showValidationError("Playlist name is too long (max 255 characters)");
      return;
    }

    mutation.mutate();
  };

  const inputIdPrefix = mode === "edit" ? "edit-" : "";

  return (
    <DialogContent className="border-slate-700 bg-slate-900 sm:max-w-md">
      <DialogHeader>
        <DialogTitle className="text-white">{config.title}</DialogTitle>
        <DialogDescription className="text-slate-400">
          {config.description}
        </DialogDescription>
      </DialogHeader>

      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="space-y-2">
          <Label
            htmlFor={`${inputIdPrefix}playlist-name`}
            className="text-slate-200"
          >
            Name <span className="text-amber-400">*</span>
          </Label>
          <Input
            id={`${inputIdPrefix}playlist-name`}
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="My Playlist"
            maxLength={255}
            className="border-slate-700 bg-slate-800 text-white placeholder:text-slate-500 focus:border-amber-500 focus:ring-amber-500"
            disabled={mutation.isPending}
            autoFocus
          />
        </div>

        <div className="space-y-2">
          <Label
            htmlFor={`${inputIdPrefix}playlist-description`}
            className="text-slate-200"
          >
            Description{" "}
            <span className="text-sm text-slate-400">(optional)</span>
          </Label>
          <textarea
            id={`${inputIdPrefix}playlist-description`}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Add a description..."
            maxLength={1000}
            rows={3}
            className="w-full resize-none rounded-md border border-slate-700 bg-slate-800 px-3 py-2 text-sm text-white placeholder:text-slate-500 focus:border-amber-500 focus:ring-1 focus:ring-amber-500 focus:outline-none disabled:opacity-50"
            disabled={mutation.isPending}
          />
        </div>

        <DialogFooter className="gap-2 sm:gap-0">
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={mutation.isPending}
            className="border-slate-600 bg-transparent text-slate-300 hover:bg-slate-800 hover:text-white"
          >
            Cancel
          </Button>
          <Button
            type="submit"
            variant="accent"
            disabled={mutation.isPending || !name.trim()}
          >
            {mutation.isPending ? (
              <>
                <Spinner className="mr-2 size-4" />
                {config.pendingText}
              </>
            ) : (
              config.submitText
            )}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  );
}
