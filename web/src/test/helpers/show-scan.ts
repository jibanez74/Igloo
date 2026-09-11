import type { ShowScanStatus } from "@/types/settings";

export function showScanStatus(overrides: Partial<ShowScanStatus> = {}): ShowScanStatus {
  return {
    run_id: "test-run", state: "running", phase: "local",
    started_at: "2026-09-10T10:00:00Z", updated_at: "2026-09-10T10:00:00Z", finished_at: null,
    active_files: [], total: 316, processed: 0, imported: 0, updated: 0, unchanged: 0,
    failed: 0, deferred: 0, deleted: 0, episodes: 0, enrichment_total: 0,
    enrichment_processed: 0, enriched: 0, enrichment_failed: 0, pending_enrichment: 0,
    issue_count: 0, issues: [], ...overrides,
  };
}
