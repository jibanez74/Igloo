import { useDeferredValue, useId, useState, type RefObject } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Users } from "lucide-react";
import { createWatchRoom } from "@/lib/api";
import { WATCH_ROOMS_KEY } from "@/lib/constants";
import {
  STREAM_MODES,
  formatPlaybackAudioLabel,
  formatSubtitleLabel,
  getAvailableModes,
  getPrimaryVideoStream,
  resolvePlaybackSettings
} from "@/lib/playback";
import type { PlaybackSettings } from "@/types/playback";
import {
  movieTechnicalDetailsQueryOpts,
  watchRoomInviteUsersQueryOpts,
} from "@/lib/query-opts";
import {
  showActionFailed,
  showCreated,
  showValidationError,
} from "@/lib/toast-helpers";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";

type CreateWatchRoomDialogProps = {
  movieId: number;
  movieTitle: string;
  playbackSettings: PlaybackSettings;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  restoreFocusRef?: RefObject<HTMLElement | null>;
};

function getInitials(name: string) {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (parts.length === 0) return "?";
  if (parts.length === 1) return parts[0].slice(0, 1).toUpperCase();
  return `${parts[0].slice(0, 1)}${parts[1].slice(0, 1)}`.toUpperCase();
}

export default function CreateWatchRoomDialog({
  movieId,
  movieTitle,
  playbackSettings,
  open,
  onOpenChange,
  restoreFocusRef,
}: CreateWatchRoomDialogProps) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const searchId = useId();
  const [search, setSearch] = useState("");
  const [selectedUserIds, setSelectedUserIds] = useState<number[]>([]);
  const deferredSearch = useDeferredValue(search);

  const { data: inviteUsersData, isPending: inviteUsersPending } = useQuery(
    watchRoomInviteUsersQueryOpts(open),
  );
  const { data: techData } = useQuery(movieTechnicalDetailsQueryOpts(movieId));

  const inviteUsers =
    inviteUsersData?.error === false ? inviteUsersData.data.users : [];

  const normalizedSearch = deferredSearch.trim().toLowerCase();
  const filteredUsers =
    normalizedSearch === ""
      ? inviteUsers
      : inviteUsers.filter(user => {
          const haystacks = [user.name, user.email];
          return haystacks.some(value =>
            value.toLowerCase().includes(normalizedSearch),
          );
        });
  const selectedUsers = inviteUsers.filter(user =>
    selectedUserIds.includes(user.id),
  );

  const videoStreams = techData?.data?.video_streams ?? [];
  const audioStreams = techData?.data?.audio_streams ?? [];
  const subtitleStreams = techData?.data?.subtitles ?? [];
  const videoStream = getPrimaryVideoStream(videoStreams);
  const mimeType = techData?.data?.movie?.mime_type ?? undefined;
  const availableModes = getAvailableModes(
    videoStream?.height ?? 0,
    videoStream?.codec,
    audioStreams[0]?.codec,
    mimeType,
  );
  const resolvedSettings = resolvePlaybackSettings(
    playbackSettings,
    availableModes,
    audioStreams,
    subtitleStreams,
  );
  const modeLabel =
    STREAM_MODES.find(mode => mode.id === resolvedSettings.mode)?.label ??
    resolvedSettings.mode;
  const audioLabel =
    audioStreams[resolvedSettings.audioTrack] !== undefined
      ? formatPlaybackAudioLabel(
          audioStreams[resolvedSettings.audioTrack],
          resolvedSettings.audioTrack,
        )
      : "Default";
  const subtitleLabel =
    resolvedSettings.subtitleTrack === null
      ? "Off"
      : subtitleStreams[resolvedSettings.subtitleTrack] !== undefined
        ? formatSubtitleLabel(
            subtitleStreams[resolvedSettings.subtitleTrack],
            resolvedSettings.subtitleTrack,
          )
        : "Off";

  const mutation = useMutation({
    mutationFn: () =>
      createWatchRoom({
        movie_id: movieId,
        mode: resolvedSettings.mode,
        audio_track: resolvedSettings.audioTrack,
        subtitle_track: resolvedSettings.subtitleTrack,
        invited_user_ids: selectedUserIds,
      }),
    onSuccess: async data => {
      if (data.error) {
        showActionFailed("create watch room", data.message);
        return;
      }

      await queryClient.invalidateQueries({ queryKey: [WATCH_ROOMS_KEY] });
      showCreated(
        "Watch room",
        `"${movieTitle}" is ready to watch together.`,
      );
      handleOpenChange(false);
      await navigate({
        to: "/watch-rooms/$id",
        params: { id: data.data.room_id },
      });
    },
    onError: () => {
      showActionFailed(
        "create watch room",
        "An unexpected error occurred. Please try again.",
      );
    },
  });

  const resetForm = () => {
    setSearch("");
    setSelectedUserIds([]);
    mutation.reset();
  };

  const handleOpenChange = (next: boolean) => {
    if (!next) {
      resetForm();
    }
    onOpenChange(next);
  };

  const handleToggleUser = (userId: number, checked: boolean) => {
    setSelectedUserIds(current => {
      if (checked) {
        if (current.includes(userId)) return current;
        return [...current, userId];
      }
      return current.filter(id => id !== userId);
    });
  };

  const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (selectedUserIds.length === 0) {
      showValidationError("Select at least one person to invite.");
      return;
    }

    mutation.mutate();
  };

  const createDisabled =
    mutation.isPending || inviteUsersPending || inviteUsers.length === 0;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        className="flex max-h-[90vh] flex-col overflow-hidden border-border bg-card sm:max-w-xl"
        onCloseAutoFocus={event => {
          if (!restoreFocusRef) return;
          event.preventDefault();
          focusDialogRestoreTarget(restoreFocusRef.current);
        }}
      >
        <form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col">
          <DialogHeader>
            <DialogTitle className="text-foreground">Watch together</DialogTitle>
            <DialogDescription className="text-muted-foreground">
              Create a private room for <strong>{movieTitle}</strong>. Everyone
              you invite will use these playback settings and can control
              playback together.
            </DialogDescription>
          </DialogHeader>

          <div className="min-h-0 flex-1 space-y-5 overflow-y-auto py-4 pr-1">
            <section
              aria-labelledby="watch-room-playback-settings"
              className="rounded-lg border border-border bg-background/40 p-4"
            >
              <h3
                id="watch-room-playback-settings"
                className="text-sm font-semibold text-foreground"
              >
                Room playback presets
              </h3>
              <dl className="mt-3 grid gap-3 sm:grid-cols-3">
                <div>
                  <dt className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
                    Playback
                  </dt>
                  <dd className="mt-1 text-sm text-foreground">{modeLabel}</dd>
                </div>
                <div>
                  <dt className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
                    Audio
                  </dt>
                  <dd className="mt-1 text-sm text-foreground">{audioLabel}</dd>
                </div>
                <div>
                  <dt className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
                    Subtitles
                  </dt>
                  <dd className="mt-1 text-sm text-foreground">
                    {subtitleLabel}
                  </dd>
                </div>
              </dl>
            </section>

            <section
              aria-labelledby="watch-room-invitees"
              className="space-y-3"
            >
              <div className="flex items-center justify-between gap-3">
                <div>
                  <h3
                    id="watch-room-invitees"
                    className="text-sm font-semibold text-foreground"
                  >
                    Invite people
                  </h3>
                  <p className="text-sm text-muted-foreground">
                    Choose who can discover and join this room.
                  </p>
                </div>
                <div
                  className="inline-flex items-center gap-2 rounded-full border border-border bg-background/40 px-3 py-1 text-xs text-muted-foreground"
                  aria-live="polite"
                >
                  <Users className="size-3.5 text-muted-foreground" aria-hidden="true" />
                  {selectedUserIds.length} selected
                </div>
              </div>

              <div className="space-y-2">
                <Label htmlFor={searchId} className="text-muted-foreground">
                  Search users
                </Label>
                <Input
                  id={searchId}
                  value={search}
                  onChange={event => setSearch(event.target.value)}
                  className="border-border bg-muted text-foreground"
                  placeholder="Search by name or email"
                  autoComplete="off"
                />
              </div>

              {selectedUsers.length > 0 && (
                <div className="flex flex-wrap gap-2" aria-label="Selected users">
                  {selectedUsers.map(user => (
                    <button
                      key={user.id}
                      type="button"
                      onClick={() => handleToggleUser(user.id, false)}
                      className="inline-flex items-center gap-2 rounded-full border border-primary/30 bg-primary/10 px-3 py-1 text-xs text-primary transition-colors hover:bg-primary/20"
                      aria-label={`Remove ${user.name} from invited users`}
                    >
                      <span>{user.name}</span>
                      <span aria-hidden="true">x</span>
                    </button>
                  ))}
                </div>
              )}

              <div className="rounded-lg border border-border bg-background/30">
                {inviteUsersPending ? (
                  <div className="flex items-center justify-center gap-2 px-4 py-8 text-sm text-muted-foreground">
                    <Spinner className="size-4" aria-hidden="true" />
                    Loading users…
                  </div>
                ) : inviteUsersData?.error ? (
                  <p className="px-4 py-6 text-sm text-red-300">
                    {inviteUsersData.message ||
                      "Failed to load users. Please try again."}
                  </p>
                ) : inviteUsers.length === 0 ? (
                  <p className="px-4 py-6 text-sm text-muted-foreground">
                    No other users are available to invite yet.
                  </p>
                ) : filteredUsers.length === 0 ? (
                  <p className="px-4 py-6 text-sm text-muted-foreground">
                    No users match that search.
                  </p>
                ) : (
                  <ul className="divide-y divide-border">
                    {filteredUsers.map(user => {
                      const checked = selectedUserIds.includes(user.id);
                      return (
                        <li key={user.id}>
                          <label className="flex cursor-pointer items-center gap-3 px-4 py-3 transition-colors hover:bg-muted/50">
                            <Checkbox
                              checked={checked}
                              onCheckedChange={value =>
                                handleToggleUser(user.id, value === true)
                              }
                              aria-label={`Invite ${user.name}`}
                            />
                            <Avatar className="size-9 border border-border">
                              {user.avatar ? (
                                <AvatarImage
                                  src={`/api/static/${user.avatar}`}
                                  alt=""
                                />
                              ) : null}
                              <AvatarFallback className="bg-accent text-xs font-semibold text-foreground">
                                {getInitials(user.name)}
                              </AvatarFallback>
                            </Avatar>
                            <span className="min-w-0 flex-1">
                              <span className="block truncate text-sm font-medium text-foreground">
                                {user.name}
                              </span>
                              <span className="block truncate text-xs text-muted-foreground">
                                {user.email}
                              </span>
                            </span>
                          </label>
                        </li>
                      );
                    })}
                  </ul>
                )}
              </div>
            </section>
          </div>

          <DialogFooter className="gap-2 border-t border-border pt-4 sm:gap-0">
            <Button
              type="button"
              variant="ghost"
              className="text-muted-foreground"
              onClick={() => handleOpenChange(false)}
              disabled={mutation.isPending}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              variant="accent"
              disabled={createDisabled}
            >
              {mutation.isPending ? (
                <>
                  <Spinner className="size-4" />
                  Creating room…
                </>
              ) : (
                "Create and join room"
              )}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
