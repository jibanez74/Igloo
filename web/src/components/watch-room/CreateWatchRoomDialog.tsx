import { useState, type RefObject } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useSessionMutation } from "@/hooks/useSessionMutation";
import { useNavigate } from "@tanstack/react-router";
import { createWatchRoom } from "@/lib/api";
import {
  AUDIO_TRACK_DEFAULT_LABEL,
  PLAYBACK_SETTINGS_SUMMARY_LOADING,
  SUBTITLES_NONE_LABEL,
  WATCH_ROOMS_KEY,
} from "@/lib/constants";
import {
  formatPlaybackAudioLabel,
  formatSubtitleLabel,
  getAvailableModes,
  getPrimaryVideoStream,
  playbackModeLabel,
  resolvePlaybackSettings,
} from "@/lib/playback";
import type { PlaybackSettings } from "@/types/playback";
import {
  movieTechnicalDetailsQueryOpts,
  playbackSettingsFromResponse,
  playbackSettingsQueryOpts,
  watchRoomInviteUsersQueryOpts,
} from "@/lib/query-opts";
import {
  showActionFailed,
  showCreated,
  showValidationError,
} from "@/lib/toast-helpers";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Spinner } from "@/components/ui/spinner";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import { sessionEndedSince, sessionStamp } from "@/lib/session";
import { apiErrorMessage } from "@/lib/is-api-failure";
import WatchRoomInviteePicker from "@/components/watch-room/WatchRoomInviteePicker";

type CreateWatchRoomDialogProps = {
  movieId: number;
  movieTitle: string;
  playbackSettings: PlaybackSettings;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  restoreFocusRef?: RefObject<HTMLElement | null>;
};

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
  const [selectedUserIds, setSelectedUserIds] = useState<number[]>([]);

  const { data: inviteUsersData, isPending: inviteUsersPending } = useQuery(
    watchRoomInviteUsersQueryOpts(open),
  );
  const { data: playbackSettingsData } = useQuery(playbackSettingsQueryOpts());
  const { data: techData, isPending: techPending } = useQuery(
    movieTechnicalDetailsQueryOpts(movieId),
  );
  const techLoaded = Boolean(techData?.data);

  const inviteUsers =
    inviteUsersData?.error === false ? inviteUsersData.data.users : [];

  const videoStreams = techData?.data?.video_streams ?? [];
  const audioStreams = techData?.data?.audio_streams ?? [];
  const subtitleStreams = techData?.data?.subtitles ?? [];
  const videoStream = getPrimaryVideoStream(videoStreams);
  const mimeType = techData?.data?.movie?.mime_type ?? undefined;
  const availableModes = getAvailableModes({
    video: videoStream,
    videoStreamsLoaded: techLoaded,
    audioStreams,
    mimeType,
    maxTranscodeHeight:
      playbackSettingsFromResponse(playbackSettingsData)?.max_transcode_height,
  });
  const resolvedSettings = resolvePlaybackSettings(
    playbackSettings,
    availableModes,
    audioStreams,
    subtitleStreams,
  );
  const modeLabel = playbackModeLabel(
    resolvedSettings.mode,
    techLoaded ? audioStreams : undefined,
  );
  const audioLabel =
    audioStreams[resolvedSettings.audioTrack] !== undefined
      ? formatPlaybackAudioLabel(
          audioStreams[resolvedSettings.audioTrack],
          resolvedSettings.audioTrack,
        )
      : AUDIO_TRACK_DEFAULT_LABEL;
  const subtitleLabel =
    resolvedSettings.subtitleTrack === null
      ? SUBTITLES_NONE_LABEL
      : subtitleStreams[resolvedSettings.subtitleTrack] !== undefined
        ? formatSubtitleLabel(
            subtitleStreams[resolvedSettings.subtitleTrack],
            resolvedSettings.subtitleTrack,
          )
        : SUBTITLES_NONE_LABEL;

  const mutation = useSessionMutation({
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
        showActionFailed("create watch room", data);
        return;
      }

      // The callback starts in its own session (useSessionMutation checks);
      // a sign-out during this refetch must still stop the toast and the
      // navigation away from the login page.
      const session = sessionStamp();
      await queryClient.invalidateQueries({ queryKey: [WATCH_ROOMS_KEY] });
      if (sessionEndedSince(session)) return;
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
    setSelectedUserIds([]);
    mutation.reset();
  };

  const handleOpenChange = (next: boolean) => {
    if (!next) {
      resetForm();
    }
    onOpenChange(next);
  };

  const handleSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (selectedUserIds.length === 0) {
      showValidationError("Select at least one person to invite.");
      return;
    }

    mutation.mutate();
  };

  // Until technical details resolve, getAvailableModes optimistically keeps
  // `direct`, so submitting early would post a mode the source may not
  // support. PlaybackSettingsDialog gates its save the same way. The presets
  // section below says why the button is disabled while this holds.
  const createDisabled =
    mutation.isPending ||
    inviteUsersPending ||
    !techLoaded ||
    inviteUsers.length === 0;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent
        size="xl"
        className="flex max-h-[90vh] flex-col overflow-hidden"
        onCloseAutoFocus={event => {
          if (!restoreFocusRef) return;
          event.preventDefault();
          focusDialogRestoreTarget(restoreFocusRef.current);
        }}
      >
        <form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col">
          <DialogHeader>
            <DialogTitle>Watch together</DialogTitle>
            <DialogDescription>
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
              {techPending && !techLoaded ? (
                // The presets are unknown until the metadata lands, and
                // creation is blocked meanwhile — say so rather than leave a
                // disabled button with no stated reason.
                <p
                  role="status"
                  className="mt-3 flex items-center gap-2 text-sm text-muted-foreground"
                >
                  <Spinner aria-hidden="true" />
                  {PLAYBACK_SETTINGS_SUMMARY_LOADING}
                </p>
              ) : !techLoaded ? (
                <p className="mt-3 text-sm text-destructive">
                  {apiErrorMessage(
                    techData,
                    "Failed to load playback options. Please try again.",
                  )}
                </p>
              ) : (
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
                    <dd className="mt-1 text-sm text-foreground">
                      {audioLabel}
                    </dd>
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
              )}
            </section>

            <WatchRoomInviteePicker
              enabled={open}
              selectedUserIds={selectedUserIds}
              onSelectedUserIdsChange={setSelectedUserIds}
              description="Choose who can discover and join this room."
            />
          </div>

          <DialogFooter className="border-t border-border pt-4">
            <Button
              type="button"
              variant="ghost-muted"
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
                  <Spinner />
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
