import { describe, expect, it } from "vitest";

import {
  nearMisses,
  summaryMarkdown,
  type Report,
} from "../../../e2e/visual/near-misses";

function report(...statuses: string[]): Report {
  return {
    suites: [
      {
        title: "gallery.visual.ts",
        specs: statuses.map((status, index) => ({
          title: `shot ${index + 1}`,
          file: "gallery.visual.ts",
          tests: [{ status, results: [{ status: "passed", retry: 0 }] }],
        })),
      },
    ],
  };
}

describe("visual near misses", () => {
  it("calls a run whose tests all passed first time clean", () => {
    const summary = summaryMarkdown(nearMisses(report("expected", "expected")));

    expect(summary).toContain("None: no test failed or needed a retry");
    expect(summary).not.toContain("did not complete");
  });

  it("does not call a run whose tests were all skipped clean", () => {
    const misses = nearMisses(report("skipped", "skipped"));
    const summary = summaryMarkdown(misses);

    expect(misses.ran).toBe(0);
    expect(summary).toContain("**The run did not complete its tests:**");
    expect(summary).toContain("all were skipped");
    expect(summary).not.toContain("None:");
  });

  it("counts only the tests that ran beside skipped ones", () => {
    const misses = nearMisses(report("skipped", "expected", "flaky"));

    expect(misses.ran).toBe(2);
    expect(misses.flaky).toEqual(["gallery.visual.ts › shot 3"]);
    expect(summaryMarkdown(misses)).toContain("**Passed only on a retry:**");
  });

  it("calls a stopped run incomplete, not clean", () => {
    const stopped = report("expected");
    stopped.suites[0].specs!.push({
      title: "shot 2",
      file: "gallery.visual.ts",
      tests: [{ status: "skipped", results: [{ status: "interrupted", retry: 0 }] }],
    });
    const misses = nearMisses(stopped);
    const summary = summaryMarkdown(misses);

    expect(misses.interrupted).toEqual(["gallery.visual.ts › shot 2"]);
    expect(summary).toContain("- interrupted: gallery.visual.ts › shot 2");
    expect(summary).not.toContain("None:");
  });

  it("calls an empty report incomplete", () => {
    expect(summaryMarkdown(nearMisses({ suites: [] }))).toContain(
      "no test ran",
    );
  });
});
