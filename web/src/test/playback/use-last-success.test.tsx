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
      ({ data }: { data: Envelope | undefined }) => useLastSuccess(data),
      { initialProps: { data: undefined as Envelope | undefined } },
    );
    expect(result.current).toBeUndefined();

    rerender({ data: failure });
    expect(result.current).toBe(failure);
  });

  it("keeps the latest success through a failure that follows it", () => {
    const first = success("First");
    const second = success("Second");
    const { result, rerender } = renderHook(
      ({ data }: { data: Envelope | undefined }) => useLastSuccess(data),
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
