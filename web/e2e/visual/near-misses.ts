// Reads the visual run's JSON report (playwright.visual.config.ts) and lists
// its near misses as Markdown: tests that failed, tests that passed only on a
// retry, and shots whose exact comparison (visual-helpers.ts) matched only on
// a later attempt. flake-summary.ts prints it; the unit test lives in
// src/test/e2e/near-misses.test.ts.

type Annotation = { type: string; description?: string };
type Result = {
  status: string;
  retry: number;
  // Milliseconds; the functional run's summary (e2e/run-report.ts) reads it.
  duration?: number;
  annotations?: Annotation[];
};
type Test = {
  status: string;
  annotations?: Annotation[];
  results: Result[];
};
type Spec = { title: string; file: string; tests: Test[] };
export type Suite = { title: string; specs?: Spec[]; suites?: Suite[] };
export type Report = { suites: Suite[]; errors?: { message?: string }[] };

const RETRIED_ANNOTATION = "exact comparison retried";

export type NearMisses = {
  errors: string[];
  // Tests that ran: a skipped test compared nothing, so it is not counted.
  ran: number;
  failed: string[];
  flaky: string[];
  retriedShots: string[];
  // A stopped run (Ctrl-C, a CI cancel) marks the test it was in skipped,
  // with an attempt left "interrupted" and no report error.
  interrupted: string[];
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

export function nearMisses(report: Report): NearMisses {
  const errors = (report.errors ?? []).map(
    error => error.message?.split("\n")[0] ?? "unknown error",
  );
  let ran = 0;
  const failed: string[] = [];
  const flaky: string[] = [];
  const retriedShots: string[] = [];
  const interrupted: string[] = [];

  for (const [title, spec] of specsOf(report.suites)) {
    for (const test of spec.tests) {
      if (test.status !== "skipped") ran += 1;
      if (test.results.some(result => result.status === "interrupted")) {
        interrupted.push(title);
      }
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

  return { errors, ran, failed, flaky, retriedShots, interrupted };
}

export function summaryMarkdown({
  errors,
  ran,
  failed,
  flaky,
  retriedShots,
  interrupted,
}: NearMisses) {
  const lines = ["## Visual near misses", ""];
  // A run that never reached its tests (a web server that did not start, a
  // spec that did not load) reports errors outside any test; a stopped one
  // leaves an interrupted attempt.
  if (errors.length > 0 || ran === 0 || interrupted.length > 0) {
    lines.push("**The run did not complete its tests:**", "");
    lines.push(...errors.map(message => `- ${message}`));
    lines.push(...interrupted.map(title => `- interrupted: ${title}`));
    if (ran === 0) {
      lines.push(
        "- no test ran: the report holds no tests, or all were skipped",
      );
    }
    lines.push("");
  }
  if (
    errors.length +
      failed.length +
      flaky.length +
      retriedShots.length +
      interrupted.length ===
      0 && ran > 0
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
