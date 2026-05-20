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
  resolvePlaybackSettings,
  type PlaybackSettings,
} from "@/lib/playback";
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
        className="flex max-h-[90vh] flex-col overflow-hidden border-slate-700 bg-slate-900 sm:max-w-xl"
        onCloseAutoFocus={event => {
          const restoreTarget = restoreFocusRef?.current;
          if (!restoreTarget) return;
          event.preventDefault();
          restoreTarget.focus();
        }}
      >
        <form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col">
          <DialogHeader>
            <DialogTitle className="text-white">Watch together</DialogTitle>
            <DialogDescription className="text-slate-400">
              Create a private room for <strong>{movieTitle}</strong>. Everyone
              you invite will use these playback settings and can control
              playback together.
            </DialogDescription>
          </DialogHeader>

          <div className="min-h-0 flex-1 space-y-5 overflow-y-auto py-4 pr-1">
            <section
              aria-labelledby="watch-room-playback-settings"
              className="rounded-lg border border-slate-800 bg-slate-950/40 p-4"
            >
              <h3
                id="watch-room-playback-settings"
                className="text-sm font-semibold text-slate-100"
              >
                Room playback presets
              </h3>
              <dl className="mt-3 grid gap-3 sm:grid-cols-3">
                <div>
                  <dt className="text-xs font-medium tracking-wide text-slate-500 uppercase">
                    Playback
                  </dt>
                  <dd className="mt-1 text-sm text-slate-200">{modeLabel}</dd>
                </div>
                <div>
                  <dt className="text-xs font-medium tracking-wide text-slate-500 uppercase">
                    Audio
                  </dt>
                  <dd className="mt-1 text-sm text-slate-200">{audioLabel}</dd>
                </div>
                <div>
                  <dt className="text-xs font-medium tracking-wide text-slate-500 uppercase">
                    Subtitles
                  </dt>
                  <dd className="mt-1 text-sm text-slate-200">
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
                    className="text-sm font-semibold text-slate-100"
                  >
                    Invite people
                  </h3>
                  <p className="text-sm text-slate-400">
                    Choose who can discover and join this room.
                  </p>
                </div>
                <div
                  className="inline-flex items-center gap-2 rounded-full border border-slate-800 bg-slate-950/40 px-3 py-1 text-xs text-slate-300"
                  aria-live="polite"
                >
                  <Users className="size-3.5 text-slate-500" aria-hidden="true" />
                  {selectedUserIds.length} selected
                </div>
              </div>

              <div className="space-y-2">
                <Label htmlFor={searchId} className="text-slate-300">
                  Search users
                </Label>
                <Input
                  id={searchId}
                  value={search}
                  onChange={event => setSearch(event.target.value)}
                  className="border-slate-700 bg-slate-800 text-white"
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
                      className="inline-flex items-center gap-2 rounded-full border border-amber-500/30 bg-amber-500/10 px-3 py-1 text-xs text-amber-100 transition-colors hover:bg-amber-500/20"
                      aria-label={`Remove ${user.name} from invited users`}
                    >
                      <span>{user.name}</span>
                      <span aria-hidden="true">x</span>
                    </button>
                  ))}
                </div>
              )}

              <div className="rounded-lg border border-slate-800 bg-slate-950/30">
                {inviteUsersPending ? (
                  <div className="flex items-center justify-center gap-2 px-4 py-8 text-sm text-slate-400">
                    <Spinner className="size-4" aria-hidden="true" />
                    Loading users…
                  </div>
                ) : inviteUsersData?.error ? (
                  <p className="px-4 py-6 text-sm text-red-300">
                    {inviteUsersData.message ||
                      "Failed to load users. Please try again."}
                  </p>
                ) : inviteUsers.length === 0 ? (
                  <p className="px-4 py-6 text-sm text-slate-400">
                    No other users are available to invite yet.
                  </p>
                ) : filteredUsers.length === 0 ? (
                  <p className="px-4 py-6 text-sm text-slate-400">
                    No users match that search.
                  </p>
                ) : (
                  <ul className="divide-y divide-slate-800">
                    {filteredUsers.map(user => {
                      const checked = selectedUserIds.includes(user.id);
                      return (
                        <li key={user.id}>
                          <label className="flex cursor-pointer items-center gap-3 px-4 py-3 transition-colors hover:bg-slate-800/50">
                            <Checkbox
                              checked={checked}
                              onCheckedChange={value =>
                                handleToggleUser(user.id, value === true)
                              }
                              aria-label={`Invite ${user.name}`}
                            />
                            <Avatar className="size-9 border border-slate-700">
                              {user.avatar ? (
                                <AvatarImage
                                  src={`/api/static/${user.avatar}`}
                                  alt=""
                                />
                              ) : null}
                              <AvatarFallback className="bg-slate-700 text-xs font-semibold text-slate-200">
                                {getInitials(user.name)}
                              </AvatarFallback>
                            </Avatar>
                            <span className="min-w-0 flex-1">
                              <span className="block truncate text-sm font-medium text-slate-100">
                                {user.name}
                              </span>
                              <span className="block truncate text-xs text-slate-400">
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

          <DialogFooter className="gap-2 border-t border-slate-800 pt-4 sm:gap-0">
            <Button
              type="button"
              variant="ghost"
              className="text-slate-400"
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
