import type {
  MovieScanStatus,
  MusicScanStatus,
  ShowScanStatus,
} from "@/types/settings";

// A scan run in its local phase with nothing processed yet. The three
// libraries share this shape; each builder adds its own library size and the
// counters only that library reports.
function runningScan(total: number): MusicScanStatus {
  return {
    run_id: "test-run", state: "running", phase: "local",
    started_at: "2026-09-10T10:00:00Z", updated_at: "2026-09-10T10:00:00Z", finished_at: null,
    active_files: [], total, processed: 0, imported: 0, updated: 0, unchanged: 0,
    failed: 0, deferred: 0, deleted: 0, enrichment_total: 0, enrichment_processed: 0,
    enriched: 0, enrichment_failed: 0, enrichment_unmatched: 0,
    issue_count: 0, issues: [],
  };
}

export function movieScanStatus(overrides: Partial<MovieScanStatus> = {}): MovieScanStatus {
  return { ...runningScan(418), pending_enrichment: 0, ...overrides };
}

export function musicScanStatus(overrides: Partial<MusicScanStatus> = {}): MusicScanStatus {
  return { ...runningScan(2267), ...overrides };
}

export function showScanStatus(overrides: Partial<ShowScanStatus> = {}): ShowScanStatus {
  return { ...runningScan(316), episodes: 0, pending_enrichment: 0, ...overrides };
}
