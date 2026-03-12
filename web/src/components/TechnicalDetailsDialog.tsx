import { useRef, useCallback } from "react";
import { useQuery } from "@tanstack/react-query";
import { movieTechnicalDetailsQueryOpts } from "@/lib/query-opts";
import { unwrapString, unwrapInt } from "@/lib/nullable";
import { formatBitRate } from "@/lib/format";
import type {
  VideoStreamType,
  AudioStreamType,
  SubtitleType,
  ChapterType,
} from "@/types";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Spinner } from "@/components/ui/spinner";

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024)
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

function formatRuntime(minutes: number): string {
  const h = Math.floor(minutes / 60);
  const m = minutes % 60;
  return h > 0 ? `${h}h ${m}m` : `${m}m`;
}

function formatChapterTime(ms: number): string {
  const totalSec = Math.floor(ms / 1000);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  if (h > 0) return `${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
  return `${m}:${String(s).padStart(2, "0")}`;
}

function SectionHeading({ id, children }: { id: string; children: React.ReactNode }) {
  return (
    <h3
      id={id}
      className="mb-2 text-xs font-semibold tracking-wide text-amber-400/80 uppercase"
    >
      {children}
    </h3>
  );
}

function DetailRow({ label, value }: { label: string; value: string | null | undefined }) {
  if (!value) return null;
  return (
    <div className="flex justify-between gap-4 py-1" role="row">
      <dt className="shrink-0 text-sm text-slate-400">{label}</dt>
      <dd className="text-right text-sm text-slate-200">{value}</dd>
    </div>
  );
}

function VideoStreamCard({ stream }: { stream: VideoStreamType }) {
  const profile = unwrapString(stream.codec_profile);
  const aspect = unwrapString(stream.aspect_ratio);
  const bitDepth = unwrapInt(stream.bit_depth);
  const colorSpace = unwrapString(stream.color_space);
  const language = unwrapString(stream.language);

  const label = language
    ? `Video stream ${stream.stream_index}, ${language}`
    : `Video stream ${stream.stream_index}`;

  return (
    <div
      className="rounded-lg border border-slate-700 bg-slate-800/60 p-3 outline-none focus-visible:ring-2 focus-visible:ring-amber-500/60"
      tabIndex={0}
      aria-label={label}
    >
      <div className="mb-1 flex items-center justify-between">
        <span className="text-sm font-medium text-white">
          Stream {stream.stream_index}
        </span>
        {language && (
          <span className="rounded-sm bg-slate-700 px-1.5 py-0.5 text-xs text-slate-300 uppercase">
            {language}
          </span>
        )}
      </div>
      <dl className="space-y-0.5">
        <DetailRow label="Codec" value={profile ? `${stream.codec} (${profile})` : stream.codec} />
        <DetailRow label="Resolution" value={`${stream.width}×${stream.height}`} />
        {aspect && <DetailRow label="Aspect Ratio" value={aspect} />}
        <DetailRow label="Frame Rate" value={`${stream.frame_rate.toFixed(2)} fps`} />
        <DetailRow label="Bit Rate" value={stream.bit_rate > 0 ? formatBitRate(stream.bit_rate) : undefined} />
        {bitDepth != null && <DetailRow label="Bit Depth" value={`${bitDepth}-bit`} />}
        {colorSpace && <DetailRow label="Color Space" value={colorSpace} />}
      </dl>
    </div>
  );
}

function AudioStreamCard({ stream }: { stream: AudioStreamType }) {
  const language = unwrapString(stream.language);
  const layout = unwrapString(stream.channel_layout);
  const sampleRate = unwrapInt(stream.sample_rate);
  const profile = unwrapString(stream.codec_profile);

  const label = language
    ? `Audio stream ${stream.stream_index}, ${language}`
    : `Audio stream ${stream.stream_index}`;

  return (
    <div
      className="rounded-lg border border-slate-700 bg-slate-800/60 p-3 outline-none focus-visible:ring-2 focus-visible:ring-amber-500/60"
      tabIndex={0}
      aria-label={label}
    >
      <div className="mb-1 flex items-center justify-between">
        <span className="text-sm font-medium text-white">
          Stream {stream.stream_index}
        </span>
        {language && (
          <span className="rounded-sm bg-slate-700 px-1.5 py-0.5 text-xs text-slate-300 uppercase">
            {language}
          </span>
        )}
      </div>
      <dl className="space-y-0.5">
        <DetailRow label="Codec" value={profile ? `${stream.codec} (${profile})` : stream.codec} />
        <DetailRow label="Channels" value={layout ? `${stream.channels} (${layout})` : String(stream.channels)} />
        {sampleRate != null && <DetailRow label="Sample Rate" value={`${sampleRate} Hz`} />}
        <DetailRow label="Bit Rate" value={stream.bit_rate > 0 ? formatBitRate(stream.bit_rate) : undefined} />
      </dl>
    </div>
  );
}

function SubtitleRow({ subtitle }: { subtitle: SubtitleType }) {
  const language = unwrapString(subtitle.language);
  const title = unwrapString(subtitle.title);
  const flags = [
    subtitle.is_default && "Default",
    subtitle.is_forced && "Forced",
  ].filter(Boolean);

  const parts = [`Stream ${subtitle.stream_index}`];
  if (language) parts.push(language);
  if (title) parts.push(title);
  if (flags.length > 0) parts.push(flags.join(", "));
  const label = parts.join(", ");

  return (
    <div
      className="flex items-center justify-between rounded-lg border border-slate-700 bg-slate-800/60 px-3 py-2 outline-none focus-visible:ring-2 focus-visible:ring-amber-500/60"
      tabIndex={0}
      aria-label={label}
    >
      <div className="flex items-center gap-2">
        <span className="text-sm text-white">Stream {subtitle.stream_index}</span>
        {language && (
          <span className="rounded-sm bg-slate-700 px-1.5 py-0.5 text-xs text-slate-300 uppercase">
            {language}
          </span>
        )}
        {title && <span className="text-sm text-slate-400">{title}</span>}
      </div>
      <div className="flex items-center gap-2">
        <span className="text-xs text-slate-500">{subtitle.codec}</span>
        {flags.length > 0 && (
          <span className="text-xs text-amber-400">{flags.join(", ")}</span>
        )}
      </div>
    </div>
  );
}

function ChapterRow({ chapter }: { chapter: ChapterType }) {
  return (
    <div
      className="flex items-center justify-between rounded-lg border border-slate-700 bg-slate-800/60 px-3 py-2 outline-none focus-visible:ring-2 focus-visible:ring-amber-500/60"
      tabIndex={0}
      aria-label={`${chapter.title || "Untitled"} at ${formatChapterTime(chapter.start_time)}`}
    >
      <span className="text-sm text-white">{chapter.title || "Untitled"}</span>
      <span className="font-mono text-xs text-slate-500">
        {formatChapterTime(chapter.start_time)}
      </span>
    </div>
  );
}

export default function TechnicalDetailsDialog({
  movieId,
  open,
  onOpenChange,
}: {
  movieId: number;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const firstSectionRef = useRef<HTMLElement>(null);

  const { data, isPending, isError } = useQuery(
    movieTechnicalDetailsQueryOpts(movieId),
  );

  const details = data?.data;
  const runTime = details ? unwrapInt(details.movie.run_time) : null;

  const handleOpenAutoFocus = useCallback((e: Event) => {
    e.preventDefault();
    firstSectionRef.current?.focus();
  }, []);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className="max-h-[85vh] overflow-y-auto border-slate-700 bg-slate-900 sm:max-w-2xl"
        onOpenAutoFocus={handleOpenAutoFocus}
      >
        <DialogHeader>
          <DialogTitle className="text-white">Technical Details</DialogTitle>
          <DialogDescription className="text-slate-400">
            File and stream information from the media scanner.
          </DialogDescription>
        </DialogHeader>

        {isPending && (
          <div className="flex justify-center py-8">
            <Spinner className="size-6 text-amber-400" />
          </div>
        )}

        {isError && (
          <div className="py-8 text-center text-sm text-red-400">
            Failed to load technical details.
          </div>
        )}

        {details && (
          <div className="space-y-6">
            {/* File Info */}
            <section
              ref={firstSectionRef}
              tabIndex={0}
              aria-labelledby="td-file"
              className="outline-none focus-visible:rounded-lg focus-visible:ring-2 focus-visible:ring-amber-500/60"
            >
              <SectionHeading id="td-file">File</SectionHeading>
              <div className="rounded-lg border border-slate-700 bg-slate-800/60 p-3">
                <dl className="space-y-0.5">
                  <DetailRow label="Filename" value={details.movie.file_name} />
                  <DetailRow label="Size" value={formatFileSize(details.movie.size)} />
                  <DetailRow label="Container" value={details.movie.container.toUpperCase()} />
                  <DetailRow label="MIME Type" value={details.movie.mime_type} />
                  {runTime != null && (
                    <DetailRow label="Duration" value={formatRuntime(runTime)} />
                  )}
                </dl>
              </div>
            </section>

            {/* Video Streams */}
            {details.video_streams.length > 0 && (
              <section aria-labelledby="td-video">
                <SectionHeading id="td-video">
                  Video{details.video_streams.length > 1 ? ` (${details.video_streams.length})` : ""}
                </SectionHeading>
                <div className="space-y-2">
                  {details.video_streams.map((s) => (
                    <VideoStreamCard key={s.id} stream={s} />
                  ))}
                </div>
              </section>
            )}

            {/* Audio Streams */}
            {details.audio_streams.length > 0 && (
              <section aria-labelledby="td-audio">
                <SectionHeading id="td-audio">
                  Audio{details.audio_streams.length > 1 ? ` (${details.audio_streams.length})` : ""}
                </SectionHeading>
                <div className="space-y-2">
                  {details.audio_streams.map((s) => (
                    <AudioStreamCard key={s.id} stream={s} />
                  ))}
                </div>
              </section>
            )}

            {/* Subtitles */}
            {details.subtitles.length > 0 && (
              <section aria-labelledby="td-subtitles">
                <SectionHeading id="td-subtitles">
                  Subtitles{details.subtitles.length > 1 ? ` (${details.subtitles.length})` : ""}
                </SectionHeading>
                <div className="space-y-1.5">
                  {details.subtitles.map((s) => (
                    <SubtitleRow key={s.id} subtitle={s} />
                  ))}
                </div>
              </section>
            )}

            {/* Chapters */}
            {details.chapters.length > 0 && (
              <section aria-labelledby="td-chapters">
                <SectionHeading id="td-chapters">
                  Chapters ({details.chapters.length})
                </SectionHeading>
                <div className="space-y-1.5">
                  {details.chapters.map((c) => (
                    <ChapterRow key={c.id} chapter={c} />
                  ))}
                </div>
              </section>
            )}
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
