// Prints the functional run's summary (run-report.ts) after `bun run
// test:e2e`; CI appends it to the job summary. Failed tests are listed first,
// so a failed run is never summarised as clean.
//
//   bun e2e/run-summary.ts [test-results/e2e-results.json]
//
// Never fails the run itself.

import { existsSync, readFileSync } from "node:fs";

import { runSummaryMarkdown } from "./run-report";
import type { Report } from "./visual/near-misses";

const path = process.argv[2] ?? "test-results/e2e-results.json";
if (!existsSync(path)) {
  process.stdout.write(
    `## E2E run\n\nNo report at \`${path}\`: the e2e run did not start or did not finish.\n`,
  );
} else {
  let report: Report | null = null;
  try {
    report = JSON.parse(readFileSync(path, "utf8")) as Report;
  } catch (error) {
    // A run killed while writing leaves a partial file.
    process.stdout.write(
      `## E2E run\n\nThe report at \`${path}\` is unreadable: ${String(error)}\n`,
    );
  }
  if (report) process.stdout.write(runSummaryMarkdown(report));
}
