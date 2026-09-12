import { FOCUS_VISIBLE_RING_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";
import type { MovieScanStatus, MusicScanStatus, ShowScanStatus } from "@/types/settings";

type ScanState = MovieScanStatus["state"];

type LibraryCopy<Phase extends string> = {
  noun: string;
  lower: string;
  states: Record<ScanState, string>;
  phases: Record<Phase, string>;
  // Wording of the enrichment line: "<provider>: <processed> of <total> <attempted> · <enriched> <enrichedLabel> · …"
  enrichment: { provider: string; attempted: string; enriched: string };
  removed: string;
  unavailable: string;
  deferred: string;
};

// lower is passed rather than derived: "TV shows" keeps its capitals
// mid-sentence, where toLowerCase would produce "tv shows".
const stateLabels = (noun: string, lower: string, committed: string): Record<ScanState, string> => ({
  idle: `No ${lower} scan has run yet.`,
  running: `${noun} scan running`,
  completed: `${noun} scan completed`,
  "completed-with-issues": `${noun} scan completed with issues`,
  canceled: `${noun} scan canceled. ${committed}`,
  failed: `${noun} scan stopped after an error. ${committed}`,
});

const MOVIE_COPY: LibraryCopy<MovieScanStatus["phase"]> = {
  noun: "Movie",
  lower: "movie",
  states: stateLabels("Movie", "movie", "Committed movies are available."),
  phases: {
    idle: "No movie scan has run yet.",
    discovery: "Discovering movie files",
    local: "Inspecting and importing movies",
    "retry-wait": "Waiting for changing files to become quiet",
    cleanup: "Checking missing movies",
    enrichment: "Updating movie descriptions from TMDB",
  },
  enrichment: { provider: "Descriptions", attempted: "attempted", enriched: "updated" },
  removed: "missing movies removed",
  unavailable: "Movie scan status is unavailable. Showing the last known progress; updates will retry automatically.",
  deferred: "Files must be quiet for 60 seconds. The scan retries deferred files up to twice within two minutes; unresolved files can be retried in a later scan.",
};

const MUSIC_COPY: LibraryCopy<MusicScanStatus["phase"]> = {
  noun: "Music",
  lower: "music",
  states: stateLabels("Music", "music", "Committed tracks are available."),
  phases: {
    idle: "No music scan has run yet.",
    local: "Discovering and importing tracks",
    cleanup: "Checking missing tracks",
    enrichment: "Retrying Spotify matches",
  },
  enrichment: { provider: "Spotify", attempted: "retried", enriched: "matched" },
  removed: "missing tracks removed",
  unavailable: "Music scan status is unavailable. Showing the last known progress; updates will retry automatically.",
  deferred: "Files must be quiet for 60 seconds. Deferred files are retried on the next scan.",
};

const SHOW_COPY: LibraryCopy<ShowScanStatus["phase"]> = {
  noun: "TV shows",
  lower: "TV shows",
  states: stateLabels("TV shows", "TV shows", "Committed episodes are available."),
  phases: {
    idle: "No TV shows scan has run yet.",
    discovery: "Discovering episode files",
    local: "Inspecting and importing episodes",
    cleanup: "Checking missing episodes",
    enrichment: "Updating show descriptions from TMDB",
  },
  enrichment: { provider: "Descriptions", attempted: "attempted", enriched: "updated" },
  removed: "missing episodes removed",
  unavailable: "TV shows scan status is unavailable. Showing the last known progress; updates will retry automatically.",
  deferred: "Files must be quiet for 60 seconds. Deferred files are retried on the next scan.",
};

type Props = { unavailable: boolean } & (
  | { library: "movies"; status?: MovieScanStatus }
  | { library: "music"; status?: MusicScanStatus }
  | { library: "shows"; status?: ShowScanStatus }
);

// The discriminated props keep each library's status typed against its own
// phase enum; the copy tables carry everything else that differs.
const COPY = { movies: MOVIE_COPY, music: MUSIC_COPY, shows: SHOW_COPY };

const copyFor = (props: Props) => COPY[props.library];

const phaseLabel = (props: Props) => {
  const phase = props.status?.phase ?? "idle";
  // Every library's phase set is a subset of the movie phases, so the copy
  // table for this library always has an entry for its own status.
  return (copyFor(props).phases as Record<string, string>)[phase];
};

const enrichmentLine = (props: Props) => {
  if (!props.status) return null;
  const { provider, attempted, enriched } = copyFor(props).enrichment;
  const status = props.status;
  // Music has no pending count, so that tally is omitted rather than reported
  // as zero.
  const pending = props.library === "movies" || props.library === "shows"
    ? ` · ${props.status.pending_enrichment} pending`
    : "";
  return `${provider}: ${status.enrichment_processed} of ${status.enrichment_total} ${attempted} · ${status.enriched} ${enriched} · ${status.enrichment_failed} failed · ${status.enrichment_unmatched} unmatched${pending}`;
};

export default function ScanProgress(props: Props) {
  const { status, unavailable } = props;
  const copy = copyFor(props);
  const announcement = status?.state === "running"
    ? phaseLabel(props)
    : status ? copy.states[status.state] : `Loading ${copy.lower} scan status`;
  return (
    <div className="space-y-2 text-sm text-muted-foreground">
      <p role="status" aria-live="polite" aria-atomic="true">{announcement}</p>
      {unavailable && <p role="alert">{copy.unavailable}</p>}
      {status && status.state !== "idle" && (
        <>
          <p>{status.processed} of {status.total} files processed · {status.imported} imported · {status.updated} updated · {status.unchanged} unchanged</p>
          <p>{status.failed} failed · {status.deferred} deferred · {status.deleted} {copy.removed}</p>
          {props.library === "shows" && <p>{props.status?.episodes} local episodes</p>}
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
