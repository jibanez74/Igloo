import { describe, expect, it } from "vitest";

import {
  runSummaryMarkdown,
  SLOWEST_COUNT,
  slowestTests,
} from "../../../e2e/run-report";
import type { Report } from "../../../e2e/visual/near-misses";

type Case = { status: string; durations: number[] };

function report(...cases: Case[]): Report {
  return {
    suites: [
      {
        title: "movies.spec.ts",
        specs: cases.map(({ status, durations }, index) => ({
          title: `test ${index + 1}`,
          file: "movies.spec.ts",
          tests: [
            {
              status,
              results: durations.map((duration, retry) => ({
                status: "passed",
                retry,
                duration,
              })),
            },
          ],
        })),
      },
    ],
  };
}

describe("e2e run summary", () => {
  it("lists the slowest tests slowest first, counting every attempt", () => {
    const slowest = slowestTests(
      report(
        { status: "expected", durations: [1_000] },
        { status: "flaky", durations: [4_000, 3_000] },
        { status: "expected", durations: [5_000] },
      ),
    );

    expect(slowest).toEqual([
      { title: "movies.spec.ts › test 2", durationMs: 7_000 },
      { title: "movies.spec.ts › test 3", durationMs: 5_000 },
      { title: "movies.spec.ts › test 1", durationMs: 1_000 },
    ]);
  });

  it("keeps only the slowest few and leaves out tests that did not run", () => {
    const many = Array.from({ length: SLOWEST_COUNT + 3 }, (_, index) => ({
      status: "expected",
      durations: [(index + 1) * 100],
    }));
    const slowest = slowestTests(
      report(...many, { status: "skipped", durations: [] }),
    );

    expect(slowest).toHaveLength(SLOWEST_COUNT);
    expect(slowest[0].durationMs).toBe((SLOWEST_COUNT + 3) * 100);
    expect(slowest.map(test => test.title)).not.toContain(
      `movies.spec.ts › test ${SLOWEST_COUNT + 4}`,
    );
  });

  it("calls a run clean only when nothing failed or retried", () => {
    const clean = runSummaryMarkdown(
      report(
        { status: "expected", durations: [1_000] },
        { status: "expected", durations: [2_500] },
      ),
    );

    expect(clean).toContain("2 tests ran; none failed or needed a retry.");
    expect(clean).toContain("| movies.spec.ts › test 2 | 2.5 s |");
  });

  it("lists failed and retried tests before the timings", () => {
    const summary = runSummaryMarkdown(
      report(
        { status: "unexpected", durations: [60_000] },
        { status: "flaky", durations: [2_000, 1_000] },
      ),
    );

    expect(summary).toContain("**Failed:**\n\n- movies.spec.ts › test 1");
    expect(summary).toContain(
      "**Passed only on a retry:**\n\n- movies.spec.ts › test 2",
    );
    expect(summary).not.toContain("none failed");
    expect(summary.indexOf("**Failed:**")).toBeLessThan(
      summary.indexOf("**Slowest tests:**"),
    );
  });

  it("calls a run with no tests incomplete", () => {
    const summary = runSummaryMarkdown({ suites: [] });

    expect(summary).toContain("**The run did not complete its tests:**");
    expect(summary).not.toContain("none failed");
    expect(summary).not.toContain("**Slowest tests:**");
  });

  it("escapes a pipe in a test title so the table keeps its columns", () => {
    const summary = runSummaryMarkdown({
      suites: [
        {
          title: "a.spec.ts",
          specs: [
            {
              title: "this | that",
              file: "a.spec.ts",
              tests: [
                {
                  status: "expected",
                  results: [{ status: "passed", retry: 0, duration: 1_000 }],
                },
              ],
            },
          ],
        },
      ],
    } as Report);

    expect(summary).toContain("| a.spec.ts › this \\| that | 1.0 s |");
  });
});
