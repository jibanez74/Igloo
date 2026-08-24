import { renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { useMovieResumeDecision } from "@/hooks/useMovieResumeDecision";
import { MOVIE_WATCH_PROGRESS_MIN_SECONDS } from "@/lib/constants";

type HookProps = Parameters<typeof useMovieResumeDecision>[0];

const eligibleProps: HookProps = {
  movieId: 7,
  start: 0,
  playing: false,
  watchProgressPending: false,
  savedProgressSec: 900,
  savedDurationSec: 7200,
};

function renderDecision(initialProps: HookProps) {
  return renderHook(props => useMovieResumeDecision(props), { initialProps });
}

describe("useMovieResumeDecision", () => {
  it("offers resume once the progress query resolves with eligible progress", () => {
    const { result, rerender } = renderDecision({
      ...eligibleProps,
      watchProgressPending: true,
      savedProgressSec: null,
      savedDurationSec: null,
    });

    expect(result.current.resumeDialogOpen).toBe(false);

    rerender(eligibleProps);

    expect(result.current.resumeDialogOpen).toBe(true);
    expect(result.current.resumeTargetSec).toBe(900);
  });

  it("treats progress at the eligibility floor as resumable", () => {
    const { result } = renderDecision({
      ...eligibleProps,
      savedProgressSec: MOVIE_WATCH_PROGRESS_MIN_SECONDS,
    });

    expect(result.current.resumeDialogOpen).toBe(true);
    expect(result.current.resumeTargetSec).toBe(
      MOVIE_WATCH_PROGRESS_MIN_SECONDS,
    );
  });

  it("stays dismissed when refetched data becomes eligible later", () => {
    const { result, rerender } = renderDecision({
      ...eligibleProps,
      savedProgressSec: null,
      savedDurationSec: null,
    });

    expect(result.current.resumeDialogOpen).toBe(false);

    // Simulates a window-focus refetch returning progress saved during the
    // current playback: the latched decision must not re-open the dialog.
    rerender(eligibleProps);

    expect(result.current.resumeDialogOpen).toBe(false);
  });

  it("keeps the snapshotted resume target when saved progress changes", () => {
    const { result, rerender } = renderDecision(eligibleProps);

    expect(result.current.resumeTargetSec).toBe(900);

    rerender({ ...eligibleProps, savedProgressSec: 1500 });

    expect(result.current.resumeTargetSec).toBe(900);
  });

  it("never shows when the route already carries a start offset", () => {
    const { result, rerender } = renderDecision({
      ...eligibleProps,
      start: 900,
    });

    expect(result.current.resumeDialogOpen).toBe(false);

    rerender({ ...eligibleProps, start: 900, savedProgressSec: 1200 });

    expect(result.current.resumeDialogOpen).toBe(false);
  });

  it("dismisses when playback starts before the query resolves", () => {
    const { result, rerender } = renderDecision({
      ...eligibleProps,
      watchProgressPending: true,
      savedProgressSec: null,
      savedDurationSec: null,
    });

    rerender({
      ...eligibleProps,
      watchProgressPending: true,
      savedProgressSec: null,
      savedDurationSec: null,
      playing: true,
    });
    rerender(eligibleProps);

    expect(result.current.resumeDialogOpen).toBe(false);
  });

  it("closes after an explicit dismissal", () => {
    const { result, rerender } = renderDecision(eligibleProps);

    expect(result.current.resumeDialogOpen).toBe(true);

    result.current.dismissResumeDecision();
    rerender(eligibleProps);

    expect(result.current.resumeDialogOpen).toBe(false);
  });

  it("re-evaluates when the movie changes", () => {
    const { result, rerender } = renderDecision(eligibleProps);

    result.current.dismissResumeDecision();
    rerender(eligibleProps);
    expect(result.current.resumeDialogOpen).toBe(false);

    rerender({ ...eligibleProps, movieId: 8, savedProgressSec: 600 });

    expect(result.current.resumeDialogOpen).toBe(true);
    expect(result.current.resumeTargetSec).toBe(600);
  });
});
