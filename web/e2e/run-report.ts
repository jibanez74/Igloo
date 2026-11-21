// Reads the functional run's JSON report (playwright.config.ts) and lists, as
// Markdown, what CI's job summary should show without reading the whole log:
// tests that failed or passed only on a retry (the visual run's nearMisses),
// and the slowest tests, which are where the next timeout or flake will come
// from. run-summary.ts prints it; the unit test lives in
// src/test/e2e/run-report.test.ts.

import { nearMisses, type Report, type Suite } from "./visual/near-misses";

/** How many of the slowest tests the summary lists. */
export const SLOWEST_COUNT = 10;

export type SlowTest = { title: string; durationMs: number };

function* timedTests(
  suites: Suite[],
  path: string[] = [],
): Generator<SlowTest> {
  for (const suite of suites) {
    const inner = suite.title ? [...path, suite.title] : path;
    for (const spec of suite.specs ?? []) {
      for (const test of spec.tests) {
        // A test skipped from its body still spent its fixtures' setup.
        if (test.status === "skipped") continue;
        // Every attempt counts: a retried test cost the run all of them.
        const durationMs = test.results.reduce(
          (total, result) => total + (result.duration ?? 0),
          0,
        );
        if (durationMs > 0) {
          yield { title: [...inner, spec.title].join(" › "), durationMs };
        }
      }
    }
    yield* timedTests(suite.suites ?? [], inner);
  }
}

/** The `count` slowest tests that ran, slowest first. */
export function slowestTests(report: Report, count = SLOWEST_COUNT): SlowTest[] {
  return [...timedTests(report.suites)]
    .sort((a, b) => b.durationMs - a.durationMs)
    .slice(0, count);
}

function seconds(durationMs: number) {
  return `${(durationMs / 1000).toFixed(1)} s`;
}

export function runSummaryMarkdown(report: Report) {
  const { errors, ran, failed, flaky, interrupted } = nearMisses(report);
  const lines = ["## E2E run", ""];

  if (errors.length > 0 || ran === 0 || interrupted.length > 0) {
    lines.push("**The run did not complete its tests:**", "");
    lines.push(...errors.map(message => `- ${message}`));
    lines.push(...interrupted.map(title => `- interrupted: ${title}`));
    if (ran === 0) {
      lines.push("- no test ran: the report holds no tests, or all were skipped");
    }
    lines.push("");
  }
  if (failed.length > 0) {
    lines.push("**Failed:**", "");
    lines.push(...failed.map(title => `- ${title}`), "");
  }
  if (flaky.length > 0) {
    lines.push("**Passed only on a retry:**", "");
    lines.push(...flaky.map(title => `- ${title}`), "");
  }
  if (errors.length + failed.length + flaky.length + interrupted.length === 0 && ran > 0) {
    lines.push(`${ran} tests ran; none failed or needed a retry.`, "");
  }

  const slowest = slowestTests(report);
  if (slowest.length > 0) {
    lines.push("**Slowest tests:**", "");
    lines.push("| Test | Time |", "| --- | ---: |");
    lines.push(
      ...slowest.map(
        ({ title, durationMs }) =>
          `| ${title.replaceAll("|", "\\|")} | ${seconds(durationMs)} |`,
      ),
      "",
    );
  }
  return `${lines.join("\n")}\n`;
}
