import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { movieTechnicalDetailsQueryOpts } from "@/lib/query-opts";
import {
  AUDIO_TRACK_DEFAULT_LABEL,
  AUDIO_TRACK_SELECT_DEFAULT_VALUE,
  PLAYBACK_SETTINGS_NATIVE_SELECT_CLASS,
  PLAYBACK_SETTINGS_SELECT_CONTENT_CLASS,
  PLAYBACK_SETTINGS_SELECT_TRIGGER_CLASS,
  PLAYBACK_SETTINGS_SUMMARY_LOADING,
  SUBTITLE_TRACK_SELECT_OFF_VALUE,
  SUBTITLES_NONE_LABEL,
} from "@/lib/constants";
import { preventDialogDismissIfRadixSelectContent } from "@/lib/dialog-select";
import {
  describePlaybackExperience,
  formatPlaybackAudioLabel,
  formatSubtitleLabel,
  getAvailableModes,
  getPrimaryVideoStream,
  isBitmapSubtitleCodec,
  type StreamModeId,
  type PlaybackSettings,
} from "@/lib/playback";
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
  const validIds = availableModes.map(m => m.id) as readonly string[];
  const initialMode = validIds.includes(settings.mode)
    ? settings.mode
    : (availableModes[0]?.id ?? "direct");

  const [mode, setMode] = useState<StreamModeId>(initialMode);
  const [audioTrack, setAudioTrack] = useState(settings.audioTrack);
  const [subtitleTrack, setSubtitleTrack] = useState<number | null>(
    settings.subtitleTrack,
  );

  const handleSave = () => {
    onSave({ mode, audioTrack, subtitleTrack });
  };

  const summaryText =
    isPending && !techLoaded
      ? PLAYBACK_SETTINGS_SUMMARY_LOADING
      : describePlaybackExperience(
          mode,
          audioStreams[audioTrack],
          audioTrack,
        );

  return (
    <>
      <DialogHeader>
        <DialogTitle className="text-white">Playback Settings</DialogTitle>
        <DialogDescription className="text-slate-400">
          Choose how the movie is prepared for your browser and which soundtrack
          to use.
        </DialogDescription>
      </DialogHeader>

      <div className="space-y-5">
        <div className="space-y-2">
          <Label htmlFor="video-quality" className="text-slate-200">
            Playback
          </Label>
          {prefersCoarsePointer ? (
            <select
              id="video-quality"
              className={PLAYBACK_SETTINGS_NATIVE_SELECT_CLASS}
              value={mode}
              onChange={e => setMode(e.target.value as StreamModeId)}
              disabled={isPending && !techLoaded}
            >
              {availableModes.map(m => (
                <option key={m.id} value={m.id}>
                  {m.label}
                </option>
              ))}
            </select>
          ) : (
            <Select
              value={mode}
              onValueChange={v => setMode(v as StreamModeId)}
            >
              <SelectTrigger
                id="video-quality"
                className={PLAYBACK_SETTINGS_SELECT_TRIGGER_CLASS}
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent
                container={selectPortalContainer}
                className={PLAYBACK_SETTINGS_SELECT_CONTENT_CLASS}
              >
                {availableModes.map(m => (
                  <SelectItem
                    key={m.id}
                    value={m.id}
                    className="text-slate-200"
                  >
                    {m.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>

        <div className="space-y-2">
          <Label htmlFor="audio-track" className="text-slate-200">
            Audio Track
          </Label>
          {prefersCoarsePointer ? (
            <select
              id="audio-track"
              className={PLAYBACK_SETTINGS_NATIVE_SELECT_CLASS}
              value={String(audioTrack)}
              onChange={e => setAudioTrack(Number(e.target.value))}
              disabled={isPending && !techLoaded}
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
              onValueChange={v => setAudioTrack(Number(v))}
            >
              <SelectTrigger
                id="audio-track"
                className={PLAYBACK_SETTINGS_SELECT_TRIGGER_CLASS}
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
                        className="text-slate-200"
                      >
                        {label}
                      </SelectItem>
                    );
                  })
                ) : (
                  <SelectItem
                    value={AUDIO_TRACK_SELECT_DEFAULT_VALUE}
                    className="text-slate-200"
                  >
                    {AUDIO_TRACK_DEFAULT_LABEL}
                  </SelectItem>
                )}
              </SelectContent>
            </Select>
          )}
        </div>

        <div className="space-y-2">
          <Label htmlFor="subtitles" className="text-slate-200">
            Subtitles
          </Label>
          {prefersCoarsePointer ? (
            <select
              id="subtitles"
              className={PLAYBACK_SETTINGS_NATIVE_SELECT_CLASS}
              value={
                subtitleTrack === null
                  ? SUBTITLE_TRACK_SELECT_OFF_VALUE
                  : String(subtitleTrack)
              }
              onChange={e => {
                const v = e.target.value;
                setSubtitleTrack(
                  v === SUBTITLE_TRACK_SELECT_OFF_VALUE ? null : Number(v),
                );
              }}
              disabled={isPending && !techLoaded}
            >
              <option value={SUBTITLE_TRACK_SELECT_OFF_VALUE}>
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
                  ? SUBTITLE_TRACK_SELECT_OFF_VALUE
                  : String(subtitleTrack)
              }
              onValueChange={v =>
                setSubtitleTrack(
                  v === SUBTITLE_TRACK_SELECT_OFF_VALUE ? null : Number(v),
                )
              }
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
                  value={SUBTITLE_TRACK_SELECT_OFF_VALUE}
                  className="text-slate-200"
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
                      className={bitmap ? "text-slate-500" : "text-slate-200"}
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

      <p className="mt-1 text-sm/relaxed text-slate-400" aria-live="polite">
        {summaryText}
      </p>

      <DialogFooter className="gap-2 sm:gap-0">
        <Button
          type="button"
          variant="outline"
          onClick={onCancel}
          className="border-slate-600 bg-transparent text-slate-300 hover:bg-slate-800 hover:text-white"
        >
          Cancel
        </Button>
        <Button type="button" variant="accent" onClick={handleSave}>
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
  const sourceHeight = primaryVideo?.height ?? 0;
  const videoCodec = primaryVideo?.codec;
  const audioCodec = audioStreams[0]?.codec;
  const mimeType = data?.data?.movie?.mime_type;
  const availableModes = getAvailableModes(
    sourceHeight,
    videoCodec,
    audioCodec,
    mimeType,
  );

  const selectPortalContainer = prefersCoarsePointer
    ? undefined
    : (dialogSurfaceEl ?? undefined);

  const dialogFormKey = `${movieId}-${formResetKey}`;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        ref={setDialogSurfaceEl}
        className="border-slate-700 bg-slate-900 sm:max-w-md"
        onPointerDownOutside={preventDialogDismissIfRadixSelectContent}
        onInteractOutside={preventDialogDismissIfRadixSelectContent}
        onCloseAutoFocus={event => {
          const restoreTarget = restoreFocusRef?.current;
          if (!restoreTarget) return;

          event.preventDefault();
          restoreTarget.focus();
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
