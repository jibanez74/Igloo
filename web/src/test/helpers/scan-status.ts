import type {
  MovieScanStatus,
  MusicScanStatus,
  ScanStatuses,
  ShowScanStatus,
} from "@/types/settings";

// A scan run in its local phase with nothing processed yet. The three
// libraries share this shape; each builder adds its own library size and the
// counters only that library reports.
function runningScan(total: number): Omit<MusicScanStatus, "lookups"> {
  return {
    run_id: "test-run", state: "running", phase: "local", scope: "library",
    started_at: "2026-09-10T10:00:00Z", updated_at: "2026-09-10T10:00:00Z", finished_at: null,
    active_files: [], total, processed: 0, imported: 0, updated: 0, unchanged: 0,
    failed: 0, deferred: 0, deleted: 0, enrichment_total: 0, enrichment_processed: 0,
    enriched: 0, enrichment_failed: 0, enrichment_unmatched: 0,
    issue_count: 0, issues: [], catalog_revision: 0,
  };
}

export function movieScanStatus(overrides: Partial<MovieScanStatus> = {}): MovieScanStatus {
  return { ...runningScan(418), pending_enrichment: 0, ...overrides };
}

/** The music worker's lookups before their first pass. */
export const IDLE_LOOKUPS: MusicScanStatus["lookups"] = {
  state: "idle", stage: null, processed: 0, total: 0, matched: 0, failed: 0, unmatched: 0,
};

export function musicScanStatus(overrides: Partial<MusicScanStatus> = {}): MusicScanStatus {
  return { ...runningScan(2267), lookups: IDLE_LOOKUPS, ...overrides };
}

export function showScanStatus(overrides: Partial<ShowScanStatus> = {}): ShowScanStatus {
  return { ...runningScan(316), episodes: 0, pending_enrichment: 0, ...overrides };
}

/** Overrides that turn a scan-status builder's report into an idle one. */
export const IDLE_SCAN = {
  run_id: "",
  state: "idle",
  phase: "idle",
  total: 0,
  started_at: null,
  updated_at: null,
} as const;

/**
 * The combined scan-status payload: every library idle unless given a report.
 */
export function scanStatuses(reports: Partial<ScanStatuses> = {}): ScanStatuses {
  return {
    movies: movieScanStatus(IDLE_SCAN),
    shows: showScanStatus(IDLE_SCAN),
    music: musicScanStatus(IDLE_SCAN),
    ...reports,
  };
}
