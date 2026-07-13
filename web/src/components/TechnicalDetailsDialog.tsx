import { useRef, type RefObject } from "react";
import { useQuery } from "@tanstack/react-query";
import { movieTechnicalDetailsQueryOpts } from "@/lib/query-opts";
import { unwrapString, unwrapInt, unwrapFloat } from "@/lib/nullable";
import { formatBitRate, formatRuntimeMinutes } from "@/lib/format";
import type {
  VideoStreamType,
  AudioStreamType,
  SubtitleType,
} from "@/types";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Spinner } from "@/components/ui/spinner";
import { focusDialogRestoreTarget } from "@/hooks/useDialogFocusRestore";

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024)
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

function SectionHeading({
  id,
  children,
}: {
  id: string;
  children: React.ReactNode;
}) {
  return (
    <h3
      id={id}
      className="mb-3 text-sm font-semibold tracking-wide text-primary/90 uppercase"
    >
      {children}
    </h3>
  );
}

/** One label/value row — no landmark role; avoids “region” spam in rotor navigation. */
function DetailRow({
  label,
  value,
}: {
  label: string;
  value: string | null | undefined;
}) {
  if (!value) return null;
  return (
    <p className="m-0 flex flex-col gap-0.5 border-b border-border/50 py-2 text-sm last:border-b-0 sm:flex-row sm:justify-between sm:gap-x-4">
      <span className="text-muted-foreground">{label}</span>
      <span className="text-foreground sm:text-right">{value}</span>
    </p>
  );
}

function VideoStreamCard({ stream }: { stream: VideoStreamType }) {
  const profile = unwrapString(stream.codec_profile);
  const aspect = unwrapString(stream.aspect_ratio);
  const bitDepth = unwrapInt(stream.bit_depth);
  const colorSpace = unwrapString(stream.color_space);
  const language = unwrapString(stream.language);
  const title = unwrapString(stream.title);
  const headingId = `td-video-title-${stream.id}`;

  return (
    <div
      className="rounded-lg border border-border bg-muted/60 p-3"
      aria-labelledby={headingId}
    >
      <h4
        id={headingId}
        className="mb-2 flex flex-wrap items-center gap-2 text-base font-medium text-foreground"
      >
        <span>Video stream {stream.stream_index}</span>
        {language && (
          <span className="rounded-sm bg-accent px-1.5 py-0.5 text-xs font-normal text-muted-foreground uppercase">
            {language}
          </span>
        )}
        {title && (
          <span className="text-sm font-normal text-muted-foreground">({title})</span>
        )}
      </h4>
      <div className="space-y-0">
        <DetailRow
          label="Codec"
          value={profile ? `${stream.codec} (${profile})` : stream.codec}
        />
        <DetailRow
          label="Resolution"
          value={`${stream.width}×${stream.height}`}
        />
        {aspect && <DetailRow label="Aspect ratio" value={aspect} />}
        <DetailRow
          label="Frame rate"
          value={`${stream.frame_rate.toFixed(2)} fps`}
        />
        <DetailRow
          label="Bit rate"
          value={
            stream.bit_rate > 0 ? formatBitRate(stream.bit_rate) : undefined
          }
        />
        {bitDepth != null && <DetailRow label="Bit depth" value={`${bitDepth}-bit`} />}
        {colorSpace && <DetailRow label="Color space" value={colorSpace} />}
      </div>
    </div>
  );
}

function AudioStreamCard({ stream }: { stream: AudioStreamType }) {
  const language = unwrapString(stream.language);
  const layout = unwrapString(stream.channel_layout);
  const sampleRate = unwrapInt(stream.sample_rate);
  const profile = unwrapString(stream.codec_profile);
  const title = unwrapString(stream.title);
  const headingId = `td-audio-title-${stream.id}`;

  return (
    <div
      className="rounded-lg border border-border bg-muted/60 p-3"
      aria-labelledby={headingId}
    >
      <h4
        id={headingId}
        className="mb-2 flex flex-wrap items-center gap-2 text-base font-medium text-foreground"
      >
        <span>Audio stream {stream.stream_index}</span>
        {language && (
          <span className="rounded-sm bg-accent px-1.5 py-0.5 text-xs font-normal text-muted-foreground uppercase">
            {language}
          </span>
        )}
        {title && (
          <span className="text-sm font-normal text-muted-foreground">({title})</span>
        )}
      </h4>
      <div className="space-y-0">
        <DetailRow
          label="Codec"
          value={profile ? `${stream.codec} (${profile})` : stream.codec}
        />
        <DetailRow
          label="Channels"
          value={
            layout ? `${stream.channels} (${layout})` : String(stream.channels)
          }
        />
        {sampleRate != null && (
          <DetailRow label="Sample rate" value={`${sampleRate} Hz`} />
        )}
        <DetailRow
          label="Bit rate"
          value={
            stream.bit_rate > 0 ? formatBitRate(stream.bit_rate) : undefined
          }
        />
      </div>
    </div>
  );
}

function SubtitleCard({ subtitle }: { subtitle: SubtitleType }) {
  const language = unwrapString(subtitle.language);
  const title = unwrapString(subtitle.title);
  const flags = [
    subtitle.is_default && "Default",
    subtitle.is_forced && "Forced",
  ].filter(Boolean);

  const headingId = `td-sub-title-${subtitle.id}`;

  return (
    <div
      className="rounded-lg border border-border bg-muted/60 p-3"
      aria-labelledby={headingId}
    >
      <h4
        id={headingId}
        className="mb-2 flex flex-wrap items-center gap-2 text-base font-medium text-foreground"
      >
        <span>Subtitle stream {subtitle.stream_index}</span>
        {language && (
          <span className="rounded-sm bg-accent px-1.5 py-0.5 text-xs font-normal text-muted-foreground uppercase">
            {language}
          </span>
        )}
        {title && (
          <span className="text-sm font-normal text-muted-foreground">({title})</span>
        )}
      </h4>
      <div className="space-y-0">
        <DetailRow label="Codec" value={subtitle.codec} />
        {flags.length > 0 && (
          <DetailRow label="Flags" value={flags.join(", ")} />
        )}
      </div>
        </div>
  );
}

export default function TechnicalDetailsDialog({
  movieId,
  open,
  onOpenChange,
  restoreFocusRef,
}: {
  movieId: number;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  restoreFocusRef?: RefObject<HTMLElement | null>;
}) {
  const titleRef = useRef<HTMLHeadingElement>(null);

  const { data, isPending, isError } = useQuery(
    movieTechnicalDetailsQueryOpts(movieId),
  );

  const details = data?.data;
  const runTime = details ? unwrapInt(details.movie.run_time) : null;
  const formattedRuntime = formatRuntimeMinutes(runTime);
  const durationSec = details ? unwrapFloat(details.movie.duration) : null;

  const handleOpenAutoFocus = (e: Event) => {
    e.preventDefault();
    // Focus the title first so SR starts at the top (Close is last in DOM).
    // Do not focus body content here: its ref is missing while data is loading.
    queueMicrotask(() => {
      titleRef.current?.focus();
    });
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="max-h-[85vh] overflow-y-auto border-border bg-card sm:max-w-2xl"
        onOpenAutoFocus={handleOpenAutoFocus}
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
          <DialogTitle
            ref={titleRef}
            id="technical-details-dialog-title"
            tabIndex={-1}
            className="text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:ring-offset-2 focus-visible:ring-offset-background"
          >
            Technical details
          </DialogTitle>
          <DialogDescription className="text-muted-foreground">
            Scanner-detected file and stream information. Each line below states
            what is being described, then its value—for example, file size and
            the container format. Use headings to move between sections.
          </DialogDescription>
        </DialogHeader>

        {isPending && (
          <div className="flex justify-center py-8" role="status" aria-live="polite">
            <Spinner className="size-6 text-primary" aria-hidden="true" />
            <span className="sr-only">Loading technical details</span>
          </div>
        )}

        {isError && (
          <div className="py-8 text-center text-sm text-destructive" role="alert">
            Failed to load technical details.
          </div>
        )}

        {details && (
          <div className="space-y-8">
            <section
              aria-label="File"
              className="rounded-lg outline-none"
            >
              <SectionHeading id="td-file">File</SectionHeading>
              <div className="rounded-lg border border-border bg-muted/60 p-3">
                <div className="space-y-0">
                  <DetailRow
                    label="File name"
                    value={details.movie.file_name}
                  />
                  <DetailRow
                    label="File size"
                    value={formatFileSize(details.movie.size)}
                  />
                  <DetailRow
                    label="Container format"
                    value={details.movie.container.toUpperCase()}
                  />
                  <DetailRow
                    label="Media type (MIME)"
                    value={details.movie.mime_type}
                  />
                  {formattedRuntime && (
                    <DetailRow
                      label="Duration (rounded, for display)"
                      value={formattedRuntime}
                    />
                  )}
                  {durationSec != null && (
                    <DetailRow
                      label="Exact duration (ffprobe)"
                      value={`${durationSec.toFixed(2)} s`}
                    />
                  )}
                </div>
              </div>
            </section>

            {details.video_streams.length > 0 && (
              <section
                aria-label={
                  details.video_streams.length > 1
                    ? `Video streams, ${details.video_streams.length} streams`
                    : "Video streams"
                }
              >
                <SectionHeading id="td-video">
                  Video streams
                  {details.video_streams.length > 1
                    ? ` (${details.video_streams.length})`
                    : ""}
                </SectionHeading>
                <ul className="m-0 flex list-none flex-col gap-3 p-0">
                  {details.video_streams.map(s => (
                    <li key={s.id}>
                      <VideoStreamCard stream={s} />
                    </li>
                  ))}
                </ul>
              </section>
            )}

            {details.audio_streams.length > 0 && (
              <section
                aria-label={
                  details.audio_streams.length > 1
                    ? `Audio streams, ${details.audio_streams.length} streams`
                    : "Audio streams"
                }
              >
                <SectionHeading id="td-audio">
                  Audio streams
                  {details.audio_streams.length > 1
                    ? ` (${details.audio_streams.length})`
                    : ""}
                </SectionHeading>
                <ul className="m-0 flex list-none flex-col gap-3 p-0">
                  {details.audio_streams.map(s => (
                    <li key={s.id}>
                      <AudioStreamCard stream={s} />
                    </li>
                  ))}
                </ul>
              </section>
            )}

            {details.subtitles.length > 0 && (
              <section
                aria-label={
                  details.subtitles.length > 1
                    ? `Subtitle streams, ${details.subtitles.length} streams`
                    : "Subtitle streams"
                }
              >
                <SectionHeading id="td-subtitles">
                  Subtitle streams
                  {details.subtitles.length > 1
                    ? ` (${details.subtitles.length})`
                    : ""}
                </SectionHeading>
                <ul className="m-0 flex list-none flex-col gap-3 p-0">
                  {details.subtitles.map(s => (
                    <li key={s.id}>
                      <SubtitleCard subtitle={s} />
                    </li>
                  ))}
                </ul>
              </section>
            )}

            {details.video_streams.length === 0 &&
              details.audio_streams.length === 0 &&
              details.subtitles.length === 0 && (
                <p className="text-sm text-muted-foreground">
                  No video, audio, or subtitle streams were reported for this file.
                </p>
              )}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
