import { act, render, screen } from "@testing-library/react";
import { useQuery } from "@tanstack/react-query";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import ScanProgress from "@/components/settings/ScanProgress";
import { useScanStatus } from "@/hooks/useScanStatus";
import { ALBUMS_PAGINATED_KEY, MOVIES_STATS_KEY, MOVIES_LIBRARY_KEY, MUSIC_STATS_KEY, SHOWS_LIBRARY_KEY, SHOWS_STATS_KEY } from "@/lib/constants";
import { moviesLibraryQueryOpts, moviesStatsQueryOpts } from "@/lib/query-opts";
import { createTestQueryClient, renderWithQueryClient } from "../helpers/render";
import {
  movieScanStatus,
  musicScanStatus,
  IDLE_SCAN,
  scanStatuses,
  showScanStatus,
} from "../helpers/scan-status";
import { jsonResponse, requestURL } from "../helpers/api";

function ScanView() {
  const query = useScanStatus();
  return <ScanProgress library="movies" status={query.data?.movies} unavailable={query.isError} />;
}

// What a movie library page and its statistics keep mounted while a scan runs.
function MovieLibraryObservers() {
  useQuery(moviesLibraryQueryOpts(1));
  useQuery(moviesStatsQueryOpts());
  return null;
}

function BackgroundScanView() {
  const query = useScanStatus({ background: true });
  return <ScanProgress library="movies" status={query.data?.movies} unavailable={query.isError} />;
}

function MusicScanView() {
  const query = useScanStatus();
  return <ScanProgress library="music" status={query.data?.music} unavailable={query.isError} />;
}

function ShowScanView() {
  const query = useScanStatus();
  return <ScanProgress library="shows" status={query.data?.shows} unavailable={query.isError} />;
}

// What the authenticated shell and Settings → Libraries show: one poll, every
// library's report.
function AllScansView() {
  const query = useScanStatus();
  return (
    <>
      <ScanProgress library="movies" status={query.data?.movies} unavailable={query.isError} />
      <ScanProgress library="music" status={query.data?.music} unavailable={query.isError} />
      <ScanProgress library="shows" status={query.data?.shows} unavailable={query.isError} />
    </>
  );
}

afterEach(() => {
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("movie scan progress", () => {
  it("announces phases and partial outcomes and exposes issues to the keyboard", async () => {
    const user = userEvent.setup();
    const status = movieScanStatus({
      state: "completed-with-issues", processed: 418, imported: 416, failed: 1, deferred: 1,
      issue_count: 2, issues: [
        { filename: "copying.mkv", phase: "local", reason: "File is still changing." },
        { filename: "broken.mkv", phase: "local", reason: "Unable to probe this movie." },
      ],
    });
    render(<ScanProgress library="movies" status={status} unavailable={false} />);
    expect(screen.getByRole("status")).toHaveTextContent("Movie scan completed with issues");
    expect(screen.getByText(/418 of 418 files processed/)).toBeInTheDocument();
    expect(screen.getByText(/Files must be quiet for 60 seconds/)).toBeInTheDocument();
    await user.tab();
    expect(screen.getByText("2 outstanding issues")).toHaveFocus();
    // jsdom does not implement the native Enter action on summary; Playwright covers it.
    await user.click(screen.getByText("2 outstanding issues"));
    expect(screen.getByText(/copying.mkv: File is still changing/)).toBeVisible();
  });

  it("counts a single outstanding issue in the singular", () => {
    const status = movieScanStatus({
      state: "completed-with-issues", processed: 418, failed: 1,
      issue_count: 1, issues: [{ filename: "broken.mkv", phase: "local", reason: "Unable to probe this movie." }],
    });
    render(<ScanProgress library="movies" status={status} unavailable={false} />);
    expect(screen.getByText("1 outstanding issue", { exact: true })).toBeInTheDocument();
  });

  it("polls at two/ten seconds, retains status across navigation and failures, and pauses when hidden", async () => {
    vi.useFakeTimers();
    let status = movieScanStatus();
    let failed = false;
    const fetchMock = vi.fn(() => jsonResponse(failed
      ? { error: true, message: "Status temporarily unavailable" }
      : { error: false, data: scanStatuses({ movies: status }) }));
    vi.stubGlobal("fetch", fetchMock);
    const client = createTestQueryClient();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    const view = renderWithQueryClient(<ScanView />, { queryClient: client });
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    expect(screen.getByRole("status")).toHaveTextContent("Inspecting and importing movies");
    status = movieScanStatus({ processed: 2, imported: 2, catalog_revision: 2 });
    await act(async () => { await vi.advanceTimersByTimeAsync(2_000); });
    expect(screen.getByText(/2 of 418 files processed/)).toBeInTheDocument();
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [MOVIES_STATS_KEY] });
    failed = true;
    await act(async () => { await vi.advanceTimersByTimeAsync(2_000); });
    expect(screen.getByRole("alert")).toHaveTextContent("last known progress");
    expect(screen.getByText(/2 of 418 files processed/)).toBeInTheDocument();
    const visibility = vi.spyOn(document, "visibilityState", "get");
    visibility.mockReturnValue("hidden");
    act(() => { document.dispatchEvent(new Event("visibilitychange")); });
    const beforeHidden = fetchMock.mock.calls.length;
    await act(async () => { await vi.advanceTimersByTimeAsync(20_000); });
    expect(fetchMock).toHaveBeenCalledTimes(beforeHidden);
    failed = false;
    status = movieScanStatus({ phase: "enrichment", imported: 418, processed: 418, catalog_revision: 418 });
    visibility.mockReturnValue("visible");
    act(() => { document.dispatchEvent(new Event("visibilitychange")); });
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    expect(screen.getByRole("status")).toHaveTextContent("Updating movie descriptions");
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [MOVIES_LIBRARY_KEY] });
    view.unmount();
    renderWithQueryClient(<ScanView />, { queryClient: client });
    expect(screen.getByRole("status")).toHaveTextContent("Updating movie descriptions");
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    status = movieScanStatus({ state: "completed", phase: "enrichment", processed: 418, imported: 418 });
    await act(async () => { await vi.advanceTimersByTimeAsync(2_000); });
    expect(screen.getByRole("status")).toHaveTextContent("Movie scan completed");
    const completedCalls = fetchMock.mock.calls.length;
    await act(async () => { await vi.advanceTimersByTimeAsync(9_000); });
    expect(fetchMock).toHaveBeenCalledTimes(completedCalls);
    await act(async () => { await vi.advanceTimersByTimeAsync(1_000); });
    expect(fetchMock).toHaveBeenCalledTimes(completedCalls + 1);
    client.clear();
  });

  it("backs off to the idle pace while a running report keeps failing", async () => {
    vi.useFakeTimers();
    let failed = false;
    const fetchMock = vi.fn(() => jsonResponse(failed
      ? { error: true, message: "Status temporarily unavailable", status: 503 }
      : { error: false, data: scanStatuses({ movies: movieScanStatus() }) }));
    vi.stubGlobal("fetch", fetchMock);
    const client = createTestQueryClient();
    renderWithQueryClient(<ScanView />, { queryClient: client });
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    expect(screen.getByRole("status")).toHaveTextContent("Inspecting and importing movies");

    failed = true;
    await act(async () => { await vi.advanceTimersByTimeAsync(2_000); });
    expect(screen.getByRole("alert")).toHaveTextContent("last known progress");
    const afterFirstFailure = fetchMock.mock.calls.length;
    // The stale data still says "running"; the poll no longer treats it so.
    await act(async () => { await vi.advanceTimersByTimeAsync(9_000); });
    expect(fetchMock).toHaveBeenCalledTimes(afterFirstFailure);
    await act(async () => { await vi.advanceTimersByTimeAsync(1_000); });
    expect(fetchMock).toHaveBeenCalledTimes(afterFirstFailure + 1);
    client.clear();
  });

  it("polls in the background at a minute's pace while idle and every two seconds while a scan runs", async () => {
    vi.useFakeTimers();
    let status = movieScanStatus();
    const fetchMock = vi.fn(() => jsonResponse({ error: false, data: scanStatuses({ movies: status }) }));
    vi.stubGlobal("fetch", fetchMock);
    const client = createTestQueryClient();
    renderWithQueryClient(<BackgroundScanView />, { queryClient: client });
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("status")).toHaveTextContent("Inspecting and importing movies");
    await act(async () => { await vi.advanceTimersByTimeAsync(2_000); });
    expect(fetchMock).toHaveBeenCalledTimes(2);
    status = movieScanStatus({ state: "completed", processed: 418, imported: 418 });
    await act(async () => { await vi.advanceTimersByTimeAsync(2_000); });
    expect(screen.getByRole("status")).toHaveTextContent("Movie scan completed");
    const completedCalls = fetchMock.mock.calls.length;
    await act(async () => { await vi.advanceTimersByTimeAsync(30_000); });
    expect(fetchMock).toHaveBeenCalledTimes(completedCalls);
    // A watcher run that finished between two idle polls is still seen.
    await act(async () => { await vi.advanceTimersByTimeAsync(30_000); });
    expect(fetchMock).toHaveBeenCalledTimes(completedCalls + 1);
    client.clear();
  });
});

describe("music scan progress", () => {
  it("announces music phases, provider tallies and the next-scan deferral", async () => {
    const user = userEvent.setup();
    const status = musicScanStatus({
      state: "completed-with-issues", phase: "enrichment", processed: 2267, imported: 2260, failed: 1, deferred: 6, deleted: 3,
      enrichment_total: 4, enrichment_processed: 4, enriched: 120, enrichment_failed: 2, enrichment_unmatched: 9,
      issue_count: 7, issues: [{ filename: "copying.flac", phase: "local", reason: "The file is still changing." }],
    });
    const { rerender } = render(<ScanProgress library="music" status={{ ...status, state: "running" }} unavailable={false} />);
    expect(screen.getByRole("status")).toHaveTextContent("Matching music metadata");
    rerender(<ScanProgress library="music" status={status} unavailable={true} />);
    expect(screen.getByRole("status")).toHaveTextContent("Music scan completed with issues");
    expect(screen.getByRole("alert")).toHaveTextContent("Music scan status is unavailable");
    expect(screen.getByText(/2267 of 2267 files processed/)).toBeInTheDocument();
    expect(screen.getByText(/3 missing tracks removed/)).toBeInTheDocument();
    expect(screen.getByText("Catalog matching: 4 of 4 processed · 120 matched · 2 failed · 9 unmatched")).toBeInTheDocument();
    expect(screen.getByText(/retried on the next scan/)).toBeInTheDocument();
    expect(screen.queryByText(/two minutes/)).not.toBeInTheDocument();
    await user.click(screen.getByText("7 outstanding issues"));
    expect(screen.getByText(/copying.flac: The file is still changing/)).toBeVisible();
    expect(screen.getByText("Showing the first 1 issues.")).toBeInTheDocument();
  });

  it("polls the music report and refreshes music statistics and lists", async () => {
    vi.useFakeTimers();
    let status = musicScanStatus();
    const fetchMock = vi.fn((input: RequestInfo | URL) => jsonResponse(requestURL(input) === "/api/settings/scan"
      ? { error: false, data: scanStatuses({ music: status }) }
      : { error: true, message: `unexpected request ${requestURL(input)}` }));
    vi.stubGlobal("fetch", fetchMock);
    const client = createTestQueryClient();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    renderWithQueryClient(<MusicScanView />, { queryClient: client });
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    expect(screen.getByRole("status")).toHaveTextContent("Discovering and importing tracks");
    // The report seen first is the baseline: it marks the library stale but
    // refetches nothing the pages just loaded.
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [ALBUMS_PAGINATED_KEY], refetchType: "none" });
    expect(invalidate).not.toHaveBeenCalledWith({ queryKey: [ALBUMS_PAGINATED_KEY] });
    invalidate.mockClear();
    status = musicScanStatus({ processed: 54, imported: 54, active_files: ["01 Intro.m4a"], catalog_revision: 54 });
    await act(async () => { await vi.advanceTimersByTimeAsync(2_000); });
    expect(screen.getByText(/54 of 2267 files processed/)).toBeInTheDocument();
    expect(screen.getByText("Working on: 01 Intro.m4a")).toBeInTheDocument();
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [MUSIC_STATS_KEY] });
    status = musicScanStatus({ state: "completed", phase: "enrichment", processed: 2267, imported: 2267, enriched: 300, catalog_revision: 2567 });
    await act(async () => { await vi.advanceTimersByTimeAsync(2_000); });
    expect(screen.getByRole("status")).toHaveTextContent("Music scan completed");
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [ALBUMS_PAGINATED_KEY] });
    client.clear();
  });
});

describe("TV shows scan progress", () => {
  it("announces TV phases, the local episode count and the TMDB tallies", async () => {
    const user = userEvent.setup();
    const status = showScanStatus({
      state: "completed-with-issues", phase: "enrichment", processed: 316, imported: 315, failed: 1, deferred: 1, deleted: 3, episodes: 348,
      enrichment_total: 5, enrichment_processed: 5, enriched: 3, enrichment_failed: 1, enrichment_unmatched: 1, pending_enrichment: 2,
      issue_count: 2, issues: [
        { filename: "Unknown (2001)", phase: "enrichment", reason: "TMDB returned no matching show." },
        { filename: "copying.mkv", phase: "local", reason: "The file is still changing." },
      ],
    });
    render(<ScanProgress library="shows" status={status} unavailable={false} />);
    expect(screen.getByRole("status")).toHaveTextContent("TV shows scan completed with issues");
    expect(screen.getByText(/316 of 316 files processed/)).toBeInTheDocument();
    expect(screen.getByText("348 local episodes")).toBeInTheDocument();
    expect(screen.getByText(/3 missing episodes removed/)).toBeInTheDocument();
    expect(screen.getByText("Descriptions: 5 of 5 attempted · 3 updated · 1 failed · 1 unmatched · 2 pending")).toBeInTheDocument();
    expect(screen.getByText(/retried on the next scan/)).toBeInTheDocument();
    await user.click(screen.getByText("2 outstanding issues"));
    expect(screen.getByText(/Unknown \(2001\): TMDB returned no matching show/)).toBeVisible();
  });

  it("polls the TV report until the scan completes", async () => {
    vi.useFakeTimers();
    let status = showScanStatus();
    const fetchMock = vi.fn((input: RequestInfo | URL) => jsonResponse(requestURL(input) === "/api/settings/scan"
      ? { error: false, data: scanStatuses({ shows: status }) }
      : { error: true, message: `unexpected request ${requestURL(input)}` }));
    vi.stubGlobal("fetch", fetchMock);
    const client = createTestQueryClient();
    renderWithQueryClient(<ShowScanView />, { queryClient: client });
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    expect(screen.getByRole("status")).toHaveTextContent("Inspecting and importing episodes");
    status = showScanStatus({ processed: 54, imported: 54, episodes: 60, active_files: ["Show.S01E01.mkv"] });
    await act(async () => { await vi.advanceTimersByTimeAsync(2_000); });
    expect(screen.getByText(/54 of 316 files processed/)).toBeInTheDocument();
    expect(screen.getByText("60 local episodes")).toBeInTheDocument();
    expect(screen.getByText("Working on: Show.S01E01.mkv")).toBeInTheDocument();
    status = showScanStatus({ state: "completed", phase: "enrichment", processed: 316, imported: 316, episodes: 348, enriched: 369 });
    await act(async () => { await vi.advanceTimersByTimeAsync(2_000); });
    expect(screen.getByRole("status")).toHaveTextContent("TV shows scan completed");
    client.clear();
  });
});

describe("runs of watcher-detected changes", () => {
  it("words a watcher run apart from a scan and drops the episode count", () => {
    const running = showScanStatus({ scope: "changes", total: 1, episodes: 1 });
    const { rerender } = render(<ScanProgress library="shows" status={running} unavailable={false} />);
    expect(screen.getByRole("status")).toHaveTextContent("Applying detected TV show changes");
    expect(screen.queryByText(/local episodes/)).not.toBeInTheDocument();

    const done = showScanStatus({ scope: "changes", state: "completed", total: 1, processed: 1, deferred: 1 });
    rerender(<ScanProgress library="shows" status={done} unavailable={false} />);
    expect(screen.getByRole("status")).toHaveTextContent("Detected TV show changes applied");
    expect(screen.getByText(/retried by the library watcher/)).toBeInTheDocument();
    expect(screen.queryByText(/Files must be quiet for 60 seconds/)).not.toBeInTheDocument();
  });

  it("announces a clean watcher run as applied while still listing the files an earlier scan left failing", async () => {
    const user = userEvent.setup();
    const done = movieScanStatus({
      scope: "changes", state: "completed", total: 1, processed: 1, imported: 1,
      issue_count: 1, issues: [{ filename: "bad.mkv", phase: "local", reason: "The file could not be imported." }],
    });
    render(<ScanProgress library="movies" status={done} unavailable={false} />);
    expect(screen.getByRole("status")).toHaveTextContent("Detected movie changes applied");
    await user.click(screen.getByText("1 outstanding issue", { exact: true }));
    expect(screen.getByText(/bad\.mkv/)).toBeInTheDocument();
  });

  it("refreshes the library lists once a watcher run commits something, and not for one that changed nothing", async () => {
    vi.useFakeTimers();
    let status = movieScanStatus({ scope: "changes", total: 1 });
    const fetchMock = vi.fn(() => jsonResponse({ error: false, data: scanStatuses({ movies: status }) }));
    vi.stubGlobal("fetch", fetchMock);
    const client = createTestQueryClient();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    renderWithQueryClient(<ScanView />, { queryClient: client });
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    expect(invalidate).not.toHaveBeenCalledWith({ queryKey: [MOVIES_LIBRARY_KEY] });

    status = movieScanStatus({ scope: "changes", state: "completed", phase: "enrichment", total: 1, processed: 1, unchanged: 1 });
    await act(async () => { await vi.advanceTimersByTimeAsync(2_000); });
    expect(screen.getByRole("status")).toHaveTextContent("Detected movie changes applied");
    expect(invalidate).not.toHaveBeenCalledWith({ queryKey: [MOVIES_LIBRARY_KEY] });

    status = movieScanStatus({ run_id: "second-run", scope: "changes", state: "completed", phase: "enrichment", total: 1, processed: 1, imported: 1, catalog_revision: 1 });
    await act(async () => { await vi.advanceTimersByTimeAsync(10_000); });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [MOVIES_LIBRARY_KEY] });

    // A run whose files were all unchanged still refreshes the lists when an
    // enrichment retry brought metadata in.
    invalidate.mockClear();
    status = movieScanStatus({ run_id: "third-run", scope: "changes", state: "completed", phase: "enrichment", total: 1, processed: 1, unchanged: 1, enriched: 1, catalog_revision: 2 });
    await act(async () => { await vi.advanceTimersByTimeAsync(10_000); });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [MOVIES_LIBRARY_KEY] });
    client.clear();
  });

  it("only marks stale, without refetching, what the finished scan it finds at load covers", async () => {
    vi.useFakeTimers();
    let status = movieScanStatus({ run_id: "startup-scan", state: "completed", phase: "enrichment", processed: 418, imported: 418, catalog_revision: 418 });
    const library = { error: false, data: { movies: [], total: 0 } };
    const stats = { error: false, data: { total_movies: 418 } };
    const fetchMock = vi.fn((input: RequestInfo | URL) => {
      const url = requestURL(input);
      if (url === "/api/settings/scan") return jsonResponse({ error: false, data: scanStatuses({ movies: status }) });
      if (url === "/api/movies/stats") return jsonResponse(stats);
      return jsonResponse(library);
    });
    vi.stubGlobal("fetch", fetchMock);
    const requests = (path: string) => fetchMock.mock.calls.filter(([input]) => requestURL(input).split("?")[0] === path).length;
    const client = createTestQueryClient();
    // A library page and the statistics the loaders filled, both on screen;
    // the scan may have ended after they loaded.
    client.setQueryData(moviesLibraryQueryOpts(1).queryKey, library as never);
    client.setQueryData(moviesStatsQueryOpts().queryKey, stats as never);
    const invalidate = vi.spyOn(client, "invalidateQueries");
    renderWithQueryClient(<><ScanView /><MovieLibraryObservers /></>, { queryClient: client });
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    expect(screen.getByRole("status")).toHaveTextContent("Movie scan completed");
    await act(async () => { await vi.advanceTimersByTimeAsync(10_000); });

    // Stale, so its next mount or focus refetches it, but nothing refetched now.
    expect(client.getQueryState(moviesLibraryQueryOpts(1).queryKey)?.isInvalidated).toBe(true);
    expect(client.getQueryState(moviesStatsQueryOpts().queryKey)?.isInvalidated).toBe(true);
    expect(requests("/api/movies/library")).toBe(0);
    expect(requests("/api/movies/stats")).toBe(0);
    expect(invalidate).not.toHaveBeenCalledWith({ queryKey: [MOVIES_LIBRARY_KEY] });
    expect(invalidate).not.toHaveBeenCalledWith({ queryKey: [MOVIES_STATS_KEY] });

    // The next scan is news: what is on screen refetches at once.
    status = movieScanStatus({ run_id: "next-scan", catalog_revision: 418 });
    await act(async () => { await vi.advanceTimersByTimeAsync(10_000); });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [MOVIES_LIBRARY_KEY] });
    expect(requests("/api/movies/library")).toBe(1);
    expect(requests("/api/movies/stats")).toBe(1);
    client.clear();
  });

  it("refreshes for watcher runs that finished between two polls", async () => {
    vi.useFakeTimers();
    const run = (run_id: string, overrides: Parameters<typeof movieScanStatus>[0] = {}) => movieScanStatus({
      run_id, scope: "changes", state: "completed", phase: "enrichment", total: 1, processed: 1, unchanged: 1, ...overrides,
    });
    let status = run("idle-run", { catalog_revision: 4 });
    vi.stubGlobal("fetch", vi.fn(() => jsonResponse({ error: false, data: scanStatuses({ movies: status }) })));
    const client = createTestQueryClient();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    renderWithQueryClient(<ScanView />, { queryClient: client });
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    invalidate.mockClear();

    // An importing run, then a run that changed nothing: only the second
    // report is seen, but the revision counts the first.
    status = run("noop-after-import", { catalog_revision: 5 });
    await act(async () => { await vi.advanceTimersByTimeAsync(10_000); });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [MOVIES_LIBRARY_KEY] });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [MOVIES_STATS_KEY] });

    // A run whose end is hidden by the next run: the lists refresh once that
    // one ends.
    invalidate.mockClear();
    status = run("hidden-end", { state: "running", phase: "local", processed: 0, unchanged: 0, catalog_revision: 5 });
    await act(async () => { await vi.advanceTimersByTimeAsync(10_000); });
    status = run("hides-it", { state: "running", phase: "local", processed: 0, unchanged: 0, catalog_revision: 6 });
    await act(async () => { await vi.advanceTimersByTimeAsync(2_000); });
    expect(invalidate).not.toHaveBeenCalledWith({ queryKey: [MOVIES_LIBRARY_KEY] });
    status = run("hides-it", { catalog_revision: 6 });
    await act(async () => { await vi.advanceTimersByTimeAsync(2_000); });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [MOVIES_LIBRARY_KEY] });

    // A run that changed nothing leaves the revision, and the lists, alone.
    invalidate.mockClear();
    status = run("noop", { catalog_revision: 6 });
    await act(async () => { await vi.advanceTimersByTimeAsync(10_000); });
    expect(invalidate).not.toHaveBeenCalledWith({ queryKey: [MOVIES_LIBRARY_KEY] });
    expect(invalidate).not.toHaveBeenCalledWith({ queryKey: [MOVIES_STATS_KEY] });
    client.clear();
  });
});

describe("one poll for every library", () => {
  it("asks for all three reports in one request per poll", async () => {
    vi.useFakeTimers();
    const fetchMock = vi.fn<(input: RequestInfo | URL) => ReturnType<typeof jsonResponse>>(() => jsonResponse({
      error: false,
      data: scanStatuses({ movies: movieScanStatus({ state: "completed" }) }),
    }));
    vi.stubGlobal("fetch", fetchMock);
    const client = createTestQueryClient();
    renderWithQueryClient(<AllScansView />, { queryClient: client });
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(requestURL(fetchMock.mock.calls[0][0])).toBe("/api/settings/scan");
    expect(screen.getByText("Movie scan completed")).toBeInTheDocument();
    await act(async () => { await vi.advanceTimersByTimeAsync(10_000); });
    expect(fetchMock).toHaveBeenCalledTimes(2);
    client.clear();
  });

  it("polls every two seconds while any library runs, and at the idle pace once none does", async () => {
    vi.useFakeTimers();
    let music = musicScanStatus();
    const fetchMock = vi.fn(() => jsonResponse({ error: false, data: scanStatuses({ music }) }));
    vi.stubGlobal("fetch", fetchMock);
    const client = createTestQueryClient();
    renderWithQueryClient(<AllScansView />, { queryClient: client });
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    await act(async () => { await vi.advanceTimersByTimeAsync(2_000); });
    expect(fetchMock).toHaveBeenCalledTimes(2);

    music = musicScanStatus({ state: "completed", processed: 2267, imported: 2267 });
    await act(async () => { await vi.advanceTimersByTimeAsync(2_000); });
    const completedCalls = fetchMock.mock.calls.length;
    await act(async () => { await vi.advanceTimersByTimeAsync(9_000); });
    expect(fetchMock).toHaveBeenCalledTimes(completedCalls);
    await act(async () => { await vi.advanceTimersByTimeAsync(1_000); });
    expect(fetchMock).toHaveBeenCalledTimes(completedCalls + 1);
    client.clear();
  });

  it("sets each library's baseline from its own first report", async () => {
    vi.useFakeTimers();
    const fetchMock = vi.fn(() => jsonResponse({
      error: false,
      data: scanStatuses({
        movies: movieScanStatus({ run_id: "movie-run", state: "completed", catalog_revision: 5 }),
        shows: showScanStatus({ run_id: "show-run", state: "completed", catalog_revision: 3 }),
      }),
    }));
    vi.stubGlobal("fetch", fetchMock);
    const client = createTestQueryClient();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    renderWithQueryClient(<AllScansView />, { queryClient: client });
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });

    expect(invalidate).toHaveBeenCalledWith({ queryKey: [MOVIES_STATS_KEY], refetchType: "none" });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [MOVIES_LIBRARY_KEY], refetchType: "none" });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [SHOWS_STATS_KEY], refetchType: "none" });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [SHOWS_LIBRARY_KEY], refetchType: "none" });
    // The idle music report has no run and no revision: nothing to mark.
    expect(invalidate).not.toHaveBeenCalledWith(expect.objectContaining({ queryKey: [MUSIC_STATS_KEY] }));
    expect(invalidate).not.toHaveBeenCalledWith(expect.objectContaining({ queryKey: [ALBUMS_PAGINATED_KEY] }));
    for (const [filters] of invalidate.mock.calls) {
      expect(filters).toHaveProperty("refetchType", "none");
    }
    client.clear();
  });

  it("refreshes only the library whose report moved", async () => {
    vi.useFakeTimers();
    const movies = movieScanStatus({ run_id: "movie-run", state: "completed", catalog_revision: 5 });
    let music = musicScanStatus({ ...IDLE_SCAN });
    const fetchMock = vi.fn(() => jsonResponse({ error: false, data: scanStatuses({ movies, music }) }));
    vi.stubGlobal("fetch", fetchMock);
    const client = createTestQueryClient();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    renderWithQueryClient(<AllScansView />, { queryClient: client });
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    invalidate.mockClear();

    music = musicScanStatus({ run_id: "music-run", processed: 54, imported: 54, catalog_revision: 54 });
    await act(async () => { await vi.advanceTimersByTimeAsync(10_000); });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [MUSIC_STATS_KEY] });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [ALBUMS_PAGINATED_KEY] });
    expect(invalidate).not.toHaveBeenCalledWith(expect.objectContaining({ queryKey: [MOVIES_STATS_KEY] }));
    expect(invalidate).not.toHaveBeenCalledWith(expect.objectContaining({ queryKey: [MOVIES_LIBRARY_KEY] }));
    expect(invalidate).not.toHaveBeenCalledWith(expect.objectContaining({ queryKey: [SHOWS_LIBRARY_KEY] }));
    client.clear();
  });
});
