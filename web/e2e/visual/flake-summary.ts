// Prints the visual run's near misses (near-misses.ts) after
// `make test-visual-docker`: tests that passed only on a retry, and shots
// whose exact comparison (visual-helpers.ts) matched only on a later attempt.
// Both pass the run, so without this list they would be visible only by
// reading the whole log. Tests that failed outright are listed first, so a
// failed run is never summarised as clean.
//
//   bun e2e/visual/flake-summary.ts [test-results/visual-results.json]
//
// Prints nothing to fix when the run was clean; never fails the run itself.

import { existsSync, readFileSync } from "node:fs";

import { nearMisses, summaryMarkdown, type Report } from "./near-misses";

const path = process.argv[2] ?? "test-results/visual-results.json";
if (existsSync(path)) {
  const report = JSON.parse(readFileSync(path, "utf8")) as Report;
  process.stdout.write(summaryMarkdown(nearMisses(report)));
} else {
  process.stdout.write(
    `## Visual near misses\n\nNo report at \`${path}\`: the run did not finish.\n`,
  );
}
