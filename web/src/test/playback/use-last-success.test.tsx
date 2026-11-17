import { renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { useLastSuccess } from "@/hooks/useLastSuccess";

type Envelope =
  | { error: false; data: { title: string } }
  | { error: true; message: string };

const success = (title: string): Envelope => ({ error: false, data: { title } });
const failure: Envelope = { error: true, message: "Server unavailable" };

describe("useLastSuccess", () => {
  it("passes a failure through before anything has loaded", () => {
    const { result, rerender } = renderHook(
      ({ data, key = 1 }: { data: Envelope | undefined; key?: number }) => useLastSuccess(data, key),
      { initialProps: { data: undefined as Envelope | undefined } },
    );
    expect(result.current).toBeUndefined();

    rerender({ data: failure });
    expect(result.current).toBe(failure);
  });

  it("never lends one item's success to the next item's failure", () => {
    const episodeOne = success("Episode 1");
    const { result, rerender } = renderHook(
      ({ data, key }: { data: Envelope | undefined; key: number }) =>
        useLastSuccess(data, key),
      { initialProps: { data: episodeOne, key: 1 } },
    );

    // The up-next hand-off moves the route to episode 2, whose first load fails.
    rerender({ data: failure, key: 2 });
    expect(result.current).toBe(failure);

    const episodeTwo = success("Episode 2");
    rerender({ data: episodeTwo, key: 2 });
    rerender({ data: failure, key: 2 });
    expect(result.current).toBe(episodeTwo);
  });

  it("keeps the latest success through a failure that follows it", () => {
    const first = success("First");
    const second = success("Second");
    const { result, rerender } = renderHook(
      ({ data, key = 1 }: { data: Envelope | undefined; key?: number }) => useLastSuccess(data, key),
      { initialProps: { data: first } },
    );

    rerender({ data: failure });
    expect(result.current).toBe(first);

    rerender({ data: second });
    expect(result.current).toBe(second);

    rerender({ data: failure });
    expect(result.current).toBe(second);
  });
});
