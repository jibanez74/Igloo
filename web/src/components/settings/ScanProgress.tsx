import { FOCUS_VISIBLE_RING_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";
import type { MovieScanStatus, MusicScanStatus } from "@/types/settings";

type ScanState = MovieScanStatus["state"];

type LibraryCopy = {
  noun: string;
  states: Record<ScanState, string>;
  removed: string;
  unavailable: string;
  deferred: string;
};

const stateLabels = (noun: string, committed: string): Record<ScanState, string> => ({
  idle: `No ${noun.toLowerCase()} scan has run yet.`,
  running: `${noun} scan running`,
  completed: `${noun} scan completed`,
  "completed-with-issues": `${noun} scan completed with issues`,
  canceled: `${noun} scan canceled. ${committed}`,
  failed: `${noun} scan stopped after an error. ${committed}`,
});

const MOVIE_PHASE_LABELS: Record<MovieScanStatus["phase"], string> = {
  idle: "No movie scan has run yet.",
  discovery: "Discovering movie files",
  local: "Inspecting and importing movies",
  "retry-wait": "Waiting for changing files to become quiet",
  cleanup: "Checking missing movies",
  enrichment: "Updating movie descriptions from TMDB",
};

const MUSIC_PHASE_LABELS: Record<MusicScanStatus["phase"], string> = {
  idle: "No music scan has run yet.",
  local: "Discovering and importing tracks",
  cleanup: "Checking missing tracks",
  enrichment: "Retrying Spotify matches",
};

const MOVIE_COPY: LibraryCopy = {
  noun: "Movie",
  states: stateLabels("Movie", "Committed movies are available."),
  removed: "missing movies removed",
  unavailable: "Movie scan status is unavailable. Showing the last known progress; updates will retry automatically.",
  deferred: "Files must be quiet for 60 seconds. The scan retries deferred files up to twice within two minutes; unresolved files can be retried in a later scan.",
};

const MUSIC_COPY: LibraryCopy = {
  noun: "Music",
  states: stateLabels("Music", "Committed tracks are available."),
  removed: "missing tracks removed",
  unavailable: "Music scan status is unavailable. Showing the last known progress; updates will retry automatically.",
  deferred: "Files must be quiet for 60 seconds. Deferred files are retried on the next scan.",
};

type Props = { unavailable: boolean } & (
  | { library: "movies"; status?: MovieScanStatus }
  | { library: "music"; status?: MusicScanStatus }
);

const phaseLabel = (props: Props) => {
  if (props.library === "movies") return MOVIE_PHASE_LABELS[props.status?.phase ?? "idle"];
  return MUSIC_PHASE_LABELS[props.status?.phase ?? "idle"];
};

const enrichmentLine = (props: Props) => {
  if (props.library === "movies") {
    const status = props.status;
    if (!status) return null;
    return `Descriptions: ${status.enrichment_processed} of ${status.enrichment_total} attempted · ${status.enriched} updated · ${status.enrichment_failed} failed · ${status.enrichment_unmatched} unmatched · ${status.pending_enrichment} pending`;
  }
  const status = props.status;
  if (!status) return null;
  return `Spotify: ${status.enrichment_processed} of ${status.enrichment_total} retried · ${status.enriched} matched · ${status.enrichment_failed} failed · ${status.enrichment_unmatched} unmatched`;
};

export default function ScanProgress(props: Props) {
  const { status, unavailable } = props;
  const copy = props.library === "movies" ? MOVIE_COPY : MUSIC_COPY;
  const announcement = status?.state === "running"
    ? phaseLabel(props)
    : status ? copy.states[status.state] : `Loading ${copy.noun.toLowerCase()} scan status`;
  return (
    <div className="space-y-2 text-sm text-muted-foreground">
      <p role="status" aria-live="polite" aria-atomic="true">{announcement}</p>
      {unavailable && <p role="alert">{copy.unavailable}</p>}
      {status && status.state !== "idle" && (
        <>
          <p>{status.processed} of {status.total} files processed · {status.imported} imported · {status.updated} updated · {status.unchanged} unchanged</p>
          <p>{status.failed} failed · {status.deferred} deferred · {status.deleted} {copy.removed}</p>
          {status.active_files.length > 0 && <p className="wrap-anywhere">Working on: {status.active_files.join(", ")}</p>}
          <p>{enrichmentLine(props)}</p>
          {status.deferred > 0 && <p>{copy.deferred}</p>}
          {status.issue_count > 0 && (
            <details>
              <summary className={cn("cursor-pointer rounded-sm", FOCUS_VISIBLE_RING_CLASS)}>
                {status.issue_count} outstanding issues
              </summary>
              <ul className="mt-2 list-disc space-y-1 pl-5">
                {status.issues.map((issue, index) => (
                  <li key={index} className="wrap-anywhere">{issue.filename && `${issue.filename}: `}{issue.reason}</li>
                ))}
              </ul>
              {status.issue_count > status.issues.length && <p>Showing the first {status.issues.length} issues.</p>}
            </details>
          )}
        </>
      )}
    </div>
  );
}
