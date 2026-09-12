import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import ScanProgress from "@/components/settings/ScanProgress";
import { useMovieScanStatus, useMusicScanStatus, useShowScanStatus } from "@/hooks/useScanStatus";
import { ALBUMS_PAGINATED_KEY, MOVIES_STATS_KEY, MOVIES_LIBRARY_KEY, MUSIC_STATS_KEY } from "@/lib/constants";
import { createTestQueryClient } from "../helpers/render";
import { movieScanStatus } from "../helpers/movie-scan";
import { musicScanStatus } from "../helpers/music-scan";
import { showScanStatus } from "../helpers/show-scan";
import { jsonResponse, requestURL } from "../helpers/api";

function ScanView() {
  const query = useMovieScanStatus();
  return <ScanProgress library="movies" status={query.data} unavailable={query.isError} />;
}

function BackgroundScanView() {
  const query = useMovieScanStatus({ watchIdle: false });
  return <ScanProgress library="movies" status={query.data} unavailable={query.isError} />;
}

function MusicScanView() {
  const query = useMusicScanStatus();
  return <ScanProgress library="music" status={query.data} unavailable={query.isError} />;
}

function ShowScanView() {
  const query = useShowScanStatus();
  return <ScanProgress library="shows" status={query.data} unavailable={query.isError} />;
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

  it("polls at two/ten seconds, retains status across navigation and failures, and pauses when hidden", async () => {
    vi.useFakeTimers();
    let status = movieScanStatus();
    let failed = false;
    const fetchMock = vi.fn(() => jsonResponse(failed
      ? { error: true, message: "Status temporarily unavailable" }
      : { error: false, data: status }));
    vi.stubGlobal("fetch", fetchMock);
    const client = createTestQueryClient();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    const view = render(<QueryClientProvider client={client}><ScanView /></QueryClientProvider>);
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    expect(screen.getByRole("status")).toHaveTextContent("Inspecting and importing movies");
    status = movieScanStatus({ processed: 2, imported: 2 });
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
    status = movieScanStatus({ phase: "enrichment", imported: 418, processed: 418 });
    visibility.mockReturnValue("visible");
    act(() => { document.dispatchEvent(new Event("visibilitychange")); });
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    expect(screen.getByRole("status")).toHaveTextContent("Updating movie descriptions");
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [MOVIES_LIBRARY_KEY] });
    view.unmount();
    render(<QueryClientProvider client={client}><ScanView /></QueryClientProvider>);
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

  it("discovers a running scan without watching idle status, then stops once it finishes", async () => {
    vi.useFakeTimers();
    let status = movieScanStatus();
    const fetchMock = vi.fn(() => jsonResponse({ error: false, data: status }));
    vi.stubGlobal("fetch", fetchMock);
    const client = createTestQueryClient();
    render(<QueryClientProvider client={client}><BackgroundScanView /></QueryClientProvider>);
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
    client.clear();
  });
});

describe("music scan progress", () => {
  it("announces music phases, Spotify tallies and the next-scan deferral", async () => {
    const user = userEvent.setup();
    const status = musicScanStatus({
      state: "completed-with-issues", phase: "enrichment", processed: 2267, imported: 2260, failed: 1, deferred: 6, deleted: 3,
      enrichment_total: 4, enrichment_processed: 4, enriched: 120, enrichment_failed: 2, enrichment_unmatched: 9,
      issue_count: 7, issues: [{ filename: "copying.flac", phase: "local", reason: "The file is still changing." }],
    });
    render(<ScanProgress library="music" status={status} unavailable={true} />);
    expect(screen.getByRole("status")).toHaveTextContent("Music scan completed with issues");
    expect(screen.getByRole("alert")).toHaveTextContent("Music scan status is unavailable");
    expect(screen.getByText(/2267 of 2267 files processed/)).toBeInTheDocument();
    expect(screen.getByText(/3 missing tracks removed/)).toBeInTheDocument();
    expect(screen.getByText("Spotify: 4 of 4 retried · 120 matched · 2 failed · 9 unmatched")).toBeInTheDocument();
    expect(screen.getByText(/retried on the next scan/)).toBeInTheDocument();
    expect(screen.queryByText(/two minutes/)).not.toBeInTheDocument();
    await user.click(screen.getByText("7 outstanding issues"));
    expect(screen.getByText(/copying.flac: The file is still changing/)).toBeVisible();
    expect(screen.getByText("Showing the first 1 issues.")).toBeInTheDocument();
  });

  it("polls the music report and refreshes music statistics and lists", async () => {
    vi.useFakeTimers();
    let status = musicScanStatus();
    const fetchMock = vi.fn((input: RequestInfo | URL) => jsonResponse(requestURL(input) === "/api/settings/scan/music"
      ? { error: false, data: status }
      : { error: true, message: `unexpected request ${requestURL(input)}` }));
    vi.stubGlobal("fetch", fetchMock);
    const client = createTestQueryClient();
    const invalidate = vi.spyOn(client, "invalidateQueries");
    render(<QueryClientProvider client={client}><MusicScanView /></QueryClientProvider>);
    await act(async () => { await vi.advanceTimersByTimeAsync(1); });
    expect(screen.getByRole("status")).toHaveTextContent("Discovering and importing tracks");
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [ALBUMS_PAGINATED_KEY] });
    status = musicScanStatus({ processed: 54, imported: 54, active_files: ["01 Intro.m4a"] });
    await act(async () => { await vi.advanceTimersByTimeAsync(2_000); });
    expect(screen.getByText(/54 of 2267 files processed/)).toBeInTheDocument();
    expect(screen.getByText("Working on: 01 Intro.m4a")).toBeInTheDocument();
    expect(invalidate).toHaveBeenCalledWith({ queryKey: [MUSIC_STATS_KEY] });
    status = musicScanStatus({ state: "completed", phase: "enrichment", processed: 2267, imported: 2267, enriched: 300 });
    await act(async () => { await vi.advanceTimersByTimeAsync(2_000); });
    expect(screen.getByRole("status")).toHaveTextContent("Music scan completed");
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
    const fetchMock = vi.fn((input: RequestInfo | URL) => jsonResponse(requestURL(input) === "/api/settings/scan/shows"
      ? { error: false, data: status }
      : { error: true, message: `unexpected request ${requestURL(input)}` }));
    vi.stubGlobal("fetch", fetchMock);
    const client = createTestQueryClient();
    render(<QueryClientProvider client={client}><ShowScanView /></QueryClientProvider>);
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
