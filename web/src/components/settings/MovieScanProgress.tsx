import { FOCUS_VISIBLE_RING_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";
import type { MovieScanStatus } from "@/types/settings";

const PHASE_LABELS: Record<MovieScanStatus["phase"], string> = {
  idle: "No movie scan has run yet.",
  discovery: "Discovering movie files",
  local: "Inspecting and importing movies",
  "retry-wait": "Waiting for changing files to become quiet",
  cleanup: "Checking missing movies",
  enrichment: "Updating movie descriptions from TMDB",
};
const STATE_LABELS: Record<MovieScanStatus["state"], string> = {
  idle: "No movie scan has run yet.",
  running: "Movie scan running",
  completed: "Movie scan completed",
  "completed-with-issues": "Movie scan completed with issues",
  canceled: "Movie scan canceled. Committed movies are available.",
  failed: "Movie scan stopped after an error. Committed movies are available.",
};

type Props = { status?: MovieScanStatus; unavailable: boolean };

export default function MovieScanProgress({ status, unavailable }: Props) {
  const announcement = status?.state === "running"
    ? PHASE_LABELS[status.phase]
    : status ? STATE_LABELS[status.state] : "Loading movie scan status";
  return (
    <div className="space-y-2 text-sm text-muted-foreground">
      <p role="status" aria-live="polite" aria-atomic="true">{announcement}</p>
      {unavailable && (
        <p role="alert">Movie scan status is unavailable. Showing the last known progress; updates will retry automatically.</p>
      )}
      {status && status.state !== "idle" && (
        <>
          <p>{status.processed} of {status.total} files processed · {status.imported} imported · {status.updated} updated · {status.unchanged} unchanged</p>
          <p>{status.failed} failed · {status.deferred} deferred · {status.deleted} missing movies removed</p>
          {status.active_files.length > 0 && <p className="wrap-anywhere">Working on: {status.active_files.join(", ")}</p>}
          <p>Descriptions: {status.enrichment_processed} of {status.enrichment_total} attempted · {status.enriched} updated · {status.enrichment_failed} failed · {status.enrichment_unmatched} unmatched · {status.pending_enrichment} pending</p>
          {status.deferred > 0 && <p>Files must be quiet for 60 seconds. The scan retries deferred files up to twice within two minutes; unresolved files can be retried in a later scan.</p>}
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
