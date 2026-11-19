import { test as base, type Page } from "@playwright/test";
import {
  trackBrowserIssues,
  type BrowserIssueTracker,
  type TrackBrowserIssuesOptions,
} from "./e2e-browser-issues";
import { isRealInstance } from "./e2e-env";
import { takeUnexpectedApiRequests } from "./e2e-mock-api";

// The `test` every spec imports. It makes the checks that used to be opt-in
// run for every test, so a spec cannot forget them:
//
// - the mock API server starts each test from its initial state (mocked mode
//   only; a real instance keeps the restores its specs do by hand);
// - the default `page` is watched for console errors and warnings, uncaught
//   errors, failed requests and `/api/` error responses, and so is any page a
//   test adds with `browserIssues.track`;
// - every request a `mockApi` catch-all did not expect fails the test.
//
// All of it is asserted at teardown. A test that has already failed gets the
// issues attached instead, so its own error stays the one reported.

export { expect } from "@playwright/test";

/** What a whole spec or describe expects, set with `test.use`. */
export type BrowserIssueOptions =
  | TrackBrowserIssuesOptions
  /** Turns tracking off; the reason shows as an annotation in the report. */
  | { skip: string };

export type BrowserIssues = {
  /**
   * Adds what this test drives on purpose to what the default page ignores.
   * Call it before the actions that cause the issues.
   */
  ignore(options: TrackBrowserIssuesOptions): void;
  /**
   * Watches another page too, such as a second user's. The handle adds rules
   * for that page later, like `ignore` does for the default one.
   */
  track(page: Page, options?: TrackBrowserIssuesOptions): TrackedPage;
};

export type TrackedPage = Pick<BrowserIssues, "ignore">;

const NO_ISSUES = {
  consoleIssues: [],
  pageErrors: [],
  failedRequests: [],
  responseErrors: [],
};

type Fixtures = {
  browserIssueOptions: BrowserIssueOptions;
  browserIssues: BrowserIssues;
  resetMockApi: void;
};

export const test = base.extend<Fixtures>({
  browserIssueOptions: [{}, { option: true }],

  resetMockApi: [
    async ({ request }, use) => {
      if (!isRealInstance) {
        const response = await request.post("/api/e2e/reset");
        if (!response.ok()) {
          throw new Error(`Mock API reset failed: ${response.status()}`);
        }
      }
      await use();
    },
    { auto: true },
  ],

  browserIssues: [
    async ({ page, browserIssueOptions, resetMockApi }, use, testInfo) => {
      void resetMockApi;
      // A request answered after the previous test's teardown is not this
      // test's.
      takeUnexpectedApiRequests();

      if ("skip" in browserIssueOptions) {
        testInfo.annotations.push({
          type: "browser issues not tracked",
          description: browserIssueOptions.skip,
        });
        const untracked: TrackedPage = { ignore() {} };
        await use({ ignore() {}, track: () => untracked });
        return;
      }

      const trackers: BrowserIssueTracker[] = [
        trackBrowserIssues(page, browserIssueOptions),
      ];
      await use({
        ignore(options) {
          trackers[0].ignore(options);
        },
        track(other, options = {}) {
          const tracker = trackBrowserIssues(other, options);
          trackers.push(tracker);
          return tracker;
        },
      });

      const issues = {
        unexpectedApiRequests: takeUnexpectedApiRequests(),
        pages: trackers.map(tracker => tracker.issues()),
      };
      if (testInfo.status !== testInfo.expectedStatus) {
        await testInfo.attach("browser-issues.json", {
          body: JSON.stringify(issues, null, 2),
          contentType: "application/json",
        });
        return;
      }

      // One comparison, so a run reports every issue on every page at once.
      base
        .expect(issues, "browser issues and API requests no mock expected")
        .toEqual({ unexpectedApiRequests: [], pages: trackers.map(() => NO_ISSUES) });
    },
    { auto: true },
  ],
});
