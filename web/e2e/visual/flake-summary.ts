// Writes the visual run's near misses as Markdown for the CI job summary:
// tests that passed only on a retry, and shots whose exact comparison
// (visual-helpers.ts) matched only on a later attempt. Both pass the job, so
// without this list they would be visible only by reading the whole log.
// Tests that failed outright are listed first, so a failed run is never
// summarised as clean.
//
//   bun e2e/visual/flake-summary.ts [test-results/visual-results.json]
//
// Reads the JSON reporter's output (playwright.visual.config.ts). Prints
// nothing to fix when the run was clean; never fails the job itself.

import { existsSync, readFileSync } from "node:fs";

type Annotation = { type: string; description?: string };
type Result = { status: string; retry: number; annotations?: Annotation[] };
type Test = {
  status: string;
  annotations?: Annotation[];
  results: Result[];
};
type Spec = { title: string; file: string; tests: Test[] };
type Suite = { title: string; specs?: Spec[]; suites?: Suite[] };
type Report = { suites: Suite[]; errors?: { message?: string }[] };

const RETRIED_ANNOTATION = "exact comparison retried";

type NearMisses = {
  errors: string[];
  tests: number;
  failed: string[];
  flaky: string[];
  retriedShots: string[];
};

function* specsOf(
  suites: Suite[],
  path: string[] = [],
): Generator<[string, Spec]> {
  for (const suite of suites) {
    const inner = suite.title ? [...path, suite.title] : path;
    for (const spec of suite.specs ?? []) {
      yield [[...inner, spec.title].join(" › "), spec];
    }
    yield* specsOf(suite.suites ?? [], inner);
  }
}

function nearMisses(report: Report): NearMisses {
  const errors = (report.errors ?? []).map(
    error => error.message?.split("\n")[0] ?? "unknown error",
  );
  let tests = 0;
  const failed: string[] = [];
  const flaky: string[] = [];
  const retriedShots: string[] = [];

  for (const [title, spec] of specsOf(report.suites)) {
    for (const test of spec.tests) {
      tests += 1;
      if (test.status === "unexpected") failed.push(title);
      if (test.status === "flaky") flaky.push(title);
      // Playwright records runtime annotations on each result; older
      // reports put them on the test. Read both, once each.
      const seen = new Set<string>();
      const annotations = [
        ...(test.annotations ?? []),
        ...test.results.flatMap(result => result.annotations ?? []),
      ];
      for (const annotation of annotations) {
        if (annotation.type !== RETRIED_ANNOTATION) continue;
        const line = `${title}: ${annotation.description ?? ""}`;
        if (seen.has(line)) continue;
        seen.add(line);
        retriedShots.push(line);
      }
    }
  }

  return { errors, tests, failed, flaky, retriedShots };
}

function summaryMarkdown({
  errors,
  tests,
  failed,
  flaky,
  retriedShots,
}: NearMisses) {
  const lines = ["## Visual near misses", ""];
  // A run that never reached its tests (a web server that did not start, a
  // spec that did not load) reports errors outside any test.
  if (errors.length > 0 || tests === 0) {
    lines.push("**The run did not complete its tests:**", "");
    lines.push(...errors.map(message => `- ${message}`));
    if (tests === 0) lines.push("- the report holds no tests");
    lines.push("");
  }
  if (
    errors.length + failed.length + flaky.length + retriedShots.length === 0 &&
    tests > 0
  ) {
    lines.push(
      "None: no test failed or needed a retry, and every shot matched on its first attempt.",
    );
    return `${lines.join("\n")}\n`;
  }
  if (failed.length > 0) {
    lines.push("**Failed:**", "");
    lines.push(...failed.map(title => `- ${title}`), "");
  }
  if (flaky.length > 0) {
    lines.push("**Passed only on a retry:**", "");
    lines.push(...flaky.map(title => `- ${title}`), "");
  }
  if (retriedShots.length > 0) {
    lines.push("**Exact comparison matched only on a later attempt:**", "");
    lines.push(...retriedShots.map(line => `- ${line}`), "");
  }
  return `${lines.join("\n")}\n`;
}

const path = process.argv[2] ?? "test-results/visual-results.json";
if (existsSync(path)) {
  const report = JSON.parse(readFileSync(path, "utf8")) as Report;
  process.stdout.write(summaryMarkdown(nearMisses(report)));
} else {
  process.stdout.write(
    `## Visual near misses\n\nNo report at \`${path}\`: the run did not finish.\n`,
  );
}
