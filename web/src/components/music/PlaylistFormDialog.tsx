import { useState, type RefObject } from "react";
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
import {
  PLAYLIST_DESCRIPTION_MAX_LENGTH,
  PLAYLIST_DETAILS_KEY,
  PLAYLIST_NAME_MAX_LENGTH,
  PLAYLISTS_KEY,
} from "@/lib/constants";
import { unwrapString } from "@/lib/nullable";
import { codePointLength } from "@/lib/utils";
import type { NullableString } from "@/types";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";

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
  restoreFocusRef?: RefObject<HTMLElement | null>;
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
  const { open, onOpenChange, mode, restoreFocusRef } = props;

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
          restoreFocusRef={restoreFocusRef}
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
  restoreFocusRef?: RefObject<HTMLElement | null>;
};

function PlaylistForm({
  mode,
  playlist,
  onOpenChange,
  restoreFocusRef,
}: PlaylistFormProps) {
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

    if (codePointLength(name.trim()) > PLAYLIST_NAME_MAX_LENGTH) {
      showValidationError(
        `Playlist name is too long (max ${PLAYLIST_NAME_MAX_LENGTH} characters)`,
      );
      return;
    }

    if (codePointLength(description.trim()) > PLAYLIST_DESCRIPTION_MAX_LENGTH) {
      showValidationError(
        `Playlist description is too long (max ${PLAYLIST_DESCRIPTION_MAX_LENGTH} characters)`,
      );
      return;
    }

    mutation.mutate();
  };

  const inputIdPrefix = mode === "edit" ? "edit-" : "";
  const descriptionId = `${inputIdPrefix}playlist-description`;
  const descriptionLabelId = `${descriptionId}-label`;

  return (
    <DialogContent
      className="border-border bg-card sm:max-w-md"
      onCloseAutoFocus={
        restoreFocusRef
          ? event => {
              event.preventDefault();
              focusDialogRestoreTarget(restoreFocusRef.current);
            }
          : undefined
      }
    >
      <DialogHeader>
        <DialogTitle className="text-foreground">{config.title}</DialogTitle>
        <DialogDescription className="text-muted-foreground">
          {config.description}
        </DialogDescription>
      </DialogHeader>

      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="space-y-2">
          <Label
            htmlFor={`${inputIdPrefix}playlist-name`}
            className="text-foreground"
          >
            Name <span className="text-primary">*</span>
          </Label>
          <Input
            id={`${inputIdPrefix}playlist-name`}
            type="text"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="My Playlist"
            className="border-border bg-muted text-foreground placeholder:text-muted-foreground focus:border-ring focus:ring-ring"
            disabled={mutation.isPending}
            autoFocus
          />
        </div>

        <div className="space-y-2">
          <Label
            id={descriptionLabelId}
            htmlFor={descriptionId}
            className="text-foreground"
          >
            Description{" "}
            <span className="text-sm text-muted-foreground">(optional)</span>
          </Label>
          <textarea
            id={descriptionId}
            aria-labelledby={descriptionLabelId}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Add a description..."
            rows={3}
            className="w-full resize-none rounded-md border border-border bg-muted px-3 py-2 text-sm text-foreground placeholder:text-muted-foreground focus:border-ring focus:ring-1 focus:ring-ring focus:outline-hidden disabled:opacity-50"
            disabled={mutation.isPending}
          />
        </div>

        <DialogFooter className="gap-2 sm:gap-0">
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={mutation.isPending}
            className="border-border bg-transparent text-muted-foreground hover:bg-muted hover:text-foreground"
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
