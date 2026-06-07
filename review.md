Verify the following review comments.  If they are valid, then we should fix them.
In `@web/src/test/movies-route.test.tsx`:
- Line 254: The call to vi.useRealTimers() in movies-route.test.tsx appears
unnecessary because no tests call vi.useFakeTimers(); either remove the
vi.useRealTimers() cleanup line or if you intended to use fake timers, add
vi.useFakeTimers() at the start of the relevant test/setup (and restore with
vi.useRealTimers() in afterEach/teardown). Update the cleanup to match whichever
approach you choose so vi.useRealTimers() is not left as an unused defensive
call.
- Line 203: The test sets a global mock for window.scrollTo (window.scrollTo =
vi.fn()) but never restores it, causing test pollution; in the afterEach cleanup
(the existing afterEach block), restore the original by saving the original
scrollTo before mocking (e.g., const originalScrollTo = window.scrollTo) or
simply call vi.restoreAllMocks()/vi.resetAllMocks(), or explicitly set
window.scrollTo = originalScrollTo inside afterEach so the window.scrollTo mock
installed in the test is removed after each test run.