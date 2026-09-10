import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import MovieScanProgress from "@/components/settings/MovieScanProgress";
import { useMovieScanStatus } from "@/hooks/useMovieScanStatus";
import { MOVIES_STATS_KEY, MOVIES_LIBRARY_KEY } from "@/lib/constants";
import { createTestQueryClient } from "../helpers/render";
import { movieScanStatus } from "../helpers/movie-scan";
import { jsonResponse } from "../helpers/api";

function ScanView() {
  const query = useMovieScanStatus();
  return <MovieScanProgress status={query.data} unavailable={query.isError} />;
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
    render(<MovieScanProgress status={status} unavailable={false} />);
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
});
