import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { movieTechnicalDetailsQueryOpts } from "@/lib/query-opts";
import {
  AUDIO_TRACK_DEFAULT_LABEL,
  AUDIO_TRACK_MODE_NOTE,
  AUDIO_TRACK_MODE_NOTE_ID,
  AUDIO_TRACK_SELECT_DEFAULT_VALUE,
  PLAYBACK_SETTINGS_NATIVE_SELECT_CLASS,
  PLAYBACK_SETTINGS_SELECT_CONTENT_CLASS,
  PLAYBACK_SETTINGS_SELECT_TRIGGER_CLASS,
  PLAYBACK_SETTINGS_SUMMARY_LOADING,
  SUBTITLE_OFF_VALUE,
  SUBTITLES_NONE_LABEL,
  MOTION_MEDIA_DIALOG_SURFACE_CLASS,
} from "@/lib/constants";
import { preventDialogDismissIfRadixSelectContent } from "@/lib/dialog-select";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";
import { cn } from "@/lib/utils";
import {
  describePlaybackExperience,
  formatPlaybackAudioLabel,
  formatSubtitleLabel,
  getAvailableModes,
  getPrimaryVideoStream,
  isBitmapSubtitleCodec,
  resolveAudioTrackForMode,
  resolveModeForAudioTrack,
  resolvePlaybackSettings,
} from "@/lib/playback";
import type { PlaybackSettings, StreamModeId } from "@/types/playback";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import type { RefObject } from "react";
import { usePrefersCoarsePointer } from "@/hooks/use-coarse-pointer";
import type { AudioStreamType, SubtitleType } from "@/types/movies";

type PlaybackSettingsDialogProps = {
  movieId: number;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  settings: PlaybackSettings;
  onSave: (settings: PlaybackSettings) => void;
  restoreFocusRef?: RefObject<HTMLElement | null>;
  /** Increment when opening the dialog so the form remounts with fresh draft state. */
  formResetKey?: number;
};

type ModeOption = { id: StreamModeId; label: string };
const NO_PLAYBACK_MODES_LABEL = "No playback modes are available for this movie yet.";

type PlaybackSettingsDialogFormProps = {
  settings: PlaybackSettings;
  availableModes: ModeOption[];
  audioStreams: AudioStreamType[];
  subtitleStreams: SubtitleType[];
  isPending: boolean;
  techLoaded: boolean;
  prefersCoarsePointer: boolean;
  selectPortalContainer: HTMLElement | undefined;
  onSave: (settings: PlaybackSettings) => void;
  onCancel: () => void;
};

function PlaybackSettingsDialogForm({
  settings,
  availableModes,
  audioStreams,
  subtitleStreams,
  isPending,
  techLoaded,
  prefersCoarsePointer,
  selectPortalContainer,
  onSave,
  onCancel,
}: PlaybackSettingsDialogFormProps) {
  const normalizedSettings = resolvePlaybackSettings(
    settings,
    availableModes,
    audioStreams,
    subtitleStreams,
  );

  const [mode, setMode] = useState<StreamModeId | null>(
    availableModes.length === 0 ? null : normalizedSettings.mode,
  );
  const [audioTrack, setAudioTrack] = useState(normalizedSettings.audioTrack);
  const [subtitleTrack, setSubtitleTrack] = useState<number | null>(
    normalizedSettings.subtitleTrack,
  );
  const loadingSelectsDisabled = !techLoaded;
  const modeSelectDisabled = loadingSelectsDisabled || availableModes.length === 0;
  const audioSelectDisabled =
    loadingSelectsDisabled || availableModes.length === 0;
  const canSave = mode !== null;
  const modePlaceholder =
    isPending && !techLoaded
      ? PLAYBACK_SETTINGS_SUMMARY_LOADING
      : NO_PLAYBACK_MODES_LABEL;

  const handleSave = () => {
    if (!mode) return;
    onSave({ mode, audioTrack, subtitleTrack });
  };

  // The two selects constrain each other: direct play can only deliver the
  // container's first audio track, so whichever control the user just touched
  // wins and the other follows.
  const handleModeChange = (next: StreamModeId) => {
    setMode(next);
    setAudioTrack(resolveAudioTrackForMode(next, audioTrack));
  };

  const handleAudioTrackChange = (next: number) => {
    if (mode === null) return;

    setMode(resolveModeForAudioTrack(mode, next));
    setAudioTrack(next);
  };

  const showAudioTrackNote =
    availableModes.some(m => m.id === "direct") &&
    mode === "remux" &&
    audioTrack !== 0;
  const audioTrackNoteId = showAudioTrackNote
    ? AUDIO_TRACK_MODE_NOTE_ID
    : undefined;

  const summaryText =
    isPending && !techLoaded
      ? PLAYBACK_SETTINGS_SUMMARY_LOADING
      : mode === null
        ? NO_PLAYBACK_MODES_LABEL
        : describePlaybackExperience(mode, audioStreams[audioTrack], audioTrack);

  return (
    <>
      <DialogHeader>
        <DialogTitle className="text-foreground">Playback Settings</DialogTitle>
        <DialogDescription className="text-muted-foreground">
          Choose how the movie is prepared for your browser and which soundtrack
          to use.
        </DialogDescription>
      </DialogHeader>

      <div className="space-y-5">
        <div className="space-y-2">
          <Label htmlFor="playback-mode" className="text-foreground">
            Playback
          </Label>
          {prefersCoarsePointer ? (
            <select
              id="playback-mode"
              className={PLAYBACK_SETTINGS_NATIVE_SELECT_CLASS}
              value={mode ?? ""}
              onChange={e => handleModeChange(e.target.value as StreamModeId)}
              disabled={modeSelectDisabled}
              aria-describedby={audioTrackNoteId}
            >
              {availableModes.length > 0 ? (
                availableModes.map(m => (
                  <option key={m.id} value={m.id}>
                    {m.label}
                  </option>
                ))
              ) : (
                <option value="">{modePlaceholder}</option>
              )}
            </select>
          ) : (
            <Select
              value={mode ?? undefined}
              onValueChange={v => handleModeChange(v as StreamModeId)}
              disabled={modeSelectDisabled}
            >
              <SelectTrigger
                id="playback-mode"
                className={PLAYBACK_SETTINGS_SELECT_TRIGGER_CLASS}
                aria-describedby={audioTrackNoteId}
              >
                <SelectValue placeholder={modePlaceholder} />
              </SelectTrigger>
              <SelectContent
                container={selectPortalContainer}
                className={PLAYBACK_SETTINGS_SELECT_CONTENT_CLASS}
              >
                {availableModes.map(m => (
                  <SelectItem
                    key={m.id}
                    value={m.id}
                    className="text-foreground"
                  >
                    {m.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>

        <div className="space-y-2">
          <Label htmlFor="audio-track" className="text-foreground">
            Audio Track
          </Label>
          {prefersCoarsePointer ? (
            <select
              id="audio-track"
              className={PLAYBACK_SETTINGS_NATIVE_SELECT_CLASS}
              value={String(audioTrack)}
              onChange={e => handleAudioTrackChange(Number(e.target.value))}
              disabled={audioSelectDisabled}
              aria-describedby={audioTrackNoteId}
            >
              {audioStreams.length > 0 ? (
                audioStreams.map((stream, index) => {
                  const label = formatPlaybackAudioLabel(stream, index);
                  return (
                    <option key={stream.id} value={String(index)}>
                      {label}
                    </option>
                  );
                })
              ) : (
                <option value={AUDIO_TRACK_SELECT_DEFAULT_VALUE}>
                  {AUDIO_TRACK_DEFAULT_LABEL}
                </option>
              )}
            </select>
          ) : (
            <Select
              value={String(audioTrack)}
              onValueChange={v => handleAudioTrackChange(Number(v))}
              disabled={audioSelectDisabled}
            >
              <SelectTrigger
                id="audio-track"
                className={PLAYBACK_SETTINGS_SELECT_TRIGGER_CLASS}
                aria-describedby={audioTrackNoteId}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent
                container={selectPortalContainer}
                className={PLAYBACK_SETTINGS_SELECT_CONTENT_CLASS}
              >
                {audioStreams.length > 0 ? (
                  audioStreams.map((stream, index) => {
                    const label = formatPlaybackAudioLabel(stream, index);
                    return (
                      <SelectItem
                        key={stream.id}
                        value={String(index)}
                        className="text-foreground"
                      >
                        {label}
                      </SelectItem>
                    );
                  })
                ) : (
                  <SelectItem
                    value={AUDIO_TRACK_SELECT_DEFAULT_VALUE}
                    className="text-foreground"
                  >
                    {AUDIO_TRACK_DEFAULT_LABEL}
                  </SelectItem>
                )}
              </SelectContent>
            </Select>
          )}
          {showAudioTrackNote ? (
            <p
              id={AUDIO_TRACK_MODE_NOTE_ID}
              className="text-xs text-muted-foreground"
            >
              {AUDIO_TRACK_MODE_NOTE}
            </p>
          ) : null}
        </div>

        <div className="space-y-2">
          <Label htmlFor="subtitles" className="text-foreground">
            Subtitles
          </Label>
          {prefersCoarsePointer ? (
            <select
              id="subtitles"
              className={PLAYBACK_SETTINGS_NATIVE_SELECT_CLASS}
              value={
                subtitleTrack === null
                  ? SUBTITLE_OFF_VALUE
                  : String(subtitleTrack)
              }
              onChange={e => {
                const v = e.target.value;
                setSubtitleTrack(
                  v === SUBTITLE_OFF_VALUE ? null : Number(v),
                );
              }}
              disabled={loadingSelectsDisabled}
            >
              <option value={SUBTITLE_OFF_VALUE}>
                {SUBTITLES_NONE_LABEL}
              </option>
              {subtitleStreams.map((stream, index) => {
                const bitmap = isBitmapSubtitleCodec(stream.codec);
                const label = formatSubtitleLabel(stream, index);
                return (
                  <option
                    key={stream.id}
                    value={String(index)}
                    disabled={bitmap}
                  >
                    {bitmap ? `${label} (image-based)` : label}
                  </option>
                );
              })}
            </select>
          ) : (
            <Select
              value={
                subtitleTrack === null
                  ? SUBTITLE_OFF_VALUE
                  : String(subtitleTrack)
              }
              onValueChange={v =>
                setSubtitleTrack(
                  v === SUBTITLE_OFF_VALUE ? null : Number(v),
                )
              }
              disabled={loadingSelectsDisabled}
            >
              <SelectTrigger
                id="subtitles"
                className={PLAYBACK_SETTINGS_SELECT_TRIGGER_CLASS}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent
                container={selectPortalContainer}
                className={PLAYBACK_SETTINGS_SELECT_CONTENT_CLASS}
              >
                <SelectItem
                  value={SUBTITLE_OFF_VALUE}
                  className="text-foreground"
                >
                  {SUBTITLES_NONE_LABEL}
                </SelectItem>
                {subtitleStreams.map((stream, index) => {
                  const bitmap = isBitmapSubtitleCodec(stream.codec);
                  const label = formatSubtitleLabel(stream, index);
                  return (
                    <SelectItem
                      key={stream.id}
                      value={String(index)}
                      disabled={bitmap}
                      className={bitmap ? "text-muted-foreground" : "text-foreground"}
                    >
                      {bitmap ? `${label} (image-based)` : label}
                    </SelectItem>
                  );
                })}
              </SelectContent>
            </Select>
          )}
        </div>
      </div>

      <p className="mt-1 text-sm/relaxed text-muted-foreground" aria-live="polite">
        {summaryText}
      </p>

      <DialogFooter className="gap-2 sm:gap-0">
        <Button
          type="button"
          variant="outline"
          onClick={onCancel}
          className="border-border bg-transparent text-muted-foreground hover:bg-muted hover:text-foreground"
        >
          Cancel
        </Button>
        <Button
          type="button"
          variant="accent"
          onClick={handleSave}
          disabled={!canSave}
        >
          Done
        </Button>
      </DialogFooter>
    </>
  );
}

export default function PlaybackSettingsDialog({
  movieId,
  open,
  onOpenChange,
  settings,
  onSave,
  restoreFocusRef,
  formResetKey = 0,
}: PlaybackSettingsDialogProps) {
  const prefersCoarsePointer = usePrefersCoarsePointer();
  const [dialogSurfaceEl, setDialogSurfaceEl] = useState<HTMLDivElement | null>(
    null,
  );

  const { data, isPending } = useQuery(movieTechnicalDetailsQueryOpts(movieId));
  const audioStreams = data?.data?.audio_streams ?? [];
  const subtitleStreams = data?.data?.subtitles ?? [];
  const techLoaded = Boolean(data?.data);
  const videoStreams = data?.data?.video_streams ?? [];

  const primaryVideo = getPrimaryVideoStream(videoStreams);
  const mimeType = data?.data?.movie?.mime_type;
  // Without codec info getAvailableModes offers every mode; hold the list
  // empty until technical details arrive so impossible modes are never shown.
  const availableModes = techLoaded
    ? getAvailableModes({
        video: primaryVideo,
        videoStreamsLoaded: true,
        audioStreams,
        mimeType,
      })
    : [];

  const selectPortalContainer = prefersCoarsePointer
    ? undefined
    : (dialogSurfaceEl ?? undefined);

  const dialogFormKey = `${movieId}-${formResetKey}-${techLoaded ? "ready" : "loading"}`;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        ref={setDialogSurfaceEl}
        className={cn(MOTION_MEDIA_DIALOG_SURFACE_CLASS, "sm:max-w-md")}
        onPointerDownOutside={preventDialogDismissIfRadixSelectContent}
        onInteractOutside={preventDialogDismissIfRadixSelectContent}
        onCloseAutoFocus={event => {
          if (!restoreFocusRef) return;
          event.preventDefault();
          focusDialogRestoreTarget(restoreFocusRef.current);
        }}
      >
        <PlaybackSettingsDialogForm
          key={dialogFormKey}
          settings={settings}
          availableModes={availableModes}
          audioStreams={audioStreams}
          subtitleStreams={subtitleStreams}
          isPending={isPending}
          techLoaded={techLoaded}
          prefersCoarsePointer={prefersCoarsePointer}
          selectPortalContainer={selectPortalContainer}
          onSave={draft => {
            onSave(draft);
            onOpenChange(false);
          }}
          onCancel={() => onOpenChange(false)}
        />
        </DialogContent>
    </Dialog>
  );
}
