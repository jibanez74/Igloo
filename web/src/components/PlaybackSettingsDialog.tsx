import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { movieTechnicalDetailsQueryOpts } from "@/lib/query-opts";
import {
  describePlaybackExperience,
  formatPlaybackAudioLabel,
  getAvailableModes,
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

type PlaybackSettingsDialogProps = {
  movieId: number;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  settings: PlaybackSettings;
  onSave: (settings: PlaybackSettings) => void;
};

export default function PlaybackSettingsDialog({
  movieId,
  open,
  onOpenChange,
  settings,
  onSave,
}: PlaybackSettingsDialogProps) {
  const [mode, setMode] = useState<StreamModeId>(settings.mode);
  const [audioTrack, setAudioTrack] = useState(settings.audioTrack);

  const { data, isPending } = useQuery(movieTechnicalDetailsQueryOpts(movieId));
  const audioStreams = data?.data?.audio_streams ?? [];
  const techLoaded = Boolean(data?.data);
  const videoStreams = data?.data?.video_streams ?? [];

  const sourceHeight = videoStreams[0]?.height ?? 0;
  const videoCodec = videoStreams[0]?.codec;
  const audioCodec = audioStreams[0]?.codec;
  const mimeType = data?.data?.movie?.mime_type;
  const availableModes = getAvailableModes(
    sourceHeight,
    videoCodec,
    audioCodec,
    mimeType,
  );

  const [prevOpen, setPrevOpen] = useState(false);
  if (open && !prevOpen) {
    setPrevOpen(true);
    const validIds = availableModes.map(m => m.id) as readonly string[];
    setMode(
      validIds.includes(settings.mode)
        ? settings.mode
        : (availableModes[0]?.id ?? "direct"),
    );
    setAudioTrack(settings.audioTrack);
  } else if (!open && prevOpen) {
    setPrevOpen(false);
  }

  const handleSave = () => {
    onSave({ mode, audioTrack });
    onOpenChange(false);
  };

  const summaryText =
    isPending && !techLoaded
      ? "Loading playback options…"
      : describePlaybackExperience(
          mode,
          audioStreams[audioTrack],
          audioTrack,
        );

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="border-slate-700 bg-slate-900 sm:max-w-md">
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
            <Select
              value={mode}
              onValueChange={v => setMode(v as StreamModeId)}
            >
              <SelectTrigger
                id="video-quality"
                className="border-slate-700 bg-slate-800 text-white"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent className="border-slate-700 bg-slate-800">
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
          </div>

          <div className="space-y-2">
            <Label htmlFor="audio-track" className="text-slate-200">
              Audio Track
            </Label>
            <Select
              value={String(audioTrack)}
              onValueChange={v => setAudioTrack(Number(v))}
            >
              <SelectTrigger
                id="audio-track"
                className="border-slate-700 bg-slate-800 text-white"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent className="border-slate-700 bg-slate-800">
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
                  <SelectItem value="0" className="text-slate-200">
                    Default
                  </SelectItem>
                )}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label htmlFor="subtitles" className="text-slate-200">
              Subtitles
            </Label>
            <Select value="none" disabled>
              <SelectTrigger
                id="subtitles"
                className="border-slate-700 bg-slate-800 text-white"
              >
                <SelectValue />
              </SelectTrigger>
              <SelectContent className="border-slate-700 bg-slate-800">
                <SelectItem value="none" className="text-slate-200">
                  None
                </SelectItem>
              </SelectContent>
            </Select>
          </div>
        </div>

        <p
          className="mt-1 text-sm/relaxed text-slate-400"
          aria-live="polite"
        >
          {summaryText}
        </p>

        <DialogFooter className="gap-2 sm:gap-0">
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            className="border-slate-600 bg-transparent text-slate-300 hover:bg-slate-800 hover:text-white"
          >
            Cancel
          </Button>
          <Button type="button" variant="accent" onClick={handleSave}>
            Done
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
