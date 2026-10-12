import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { queryOptions } from "@tanstack/react-query";
import { createQueryClient, freshFor, STALE_LIST } from "@/lib/query-client";

const FAILURE = { error: true as const, message: "Server unavailable", status: 503 };
const SUCCESS = { error: false as const, data: { value: 1 } };

describe("freshFor", () => {
  it("keeps a success fresh for the whole staleTime", () => {
    expect(freshFor(STALE_LIST)({ state: { data: SUCCESS } })).toBe(STALE_LIST);
  });

  it("caps a failure envelope at the failure window", () => {
    expect(freshFor(STALE_LIST)({ state: { data: FAILURE } })).toBe(10_000);
  });

  it("never stretches a staleTime shorter than the failure window", () => {
    expect(freshFor(3_000)({ state: { data: FAILURE } })).toBe(3_000);
  });
});

describe("a query using freshFor", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  const fetchAfter = async (result: typeof SUCCESS | typeof FAILURE, gapMs: number) => {
    const queryClient = createQueryClient();
    const queryFn = vi.fn(async () => result);
    const options = queryOptions({
      queryKey: ["fresh-for", result.error],
      queryFn,
      staleTime: freshFor(STALE_LIST),
    });

    await queryClient.fetchQuery(options);
    vi.advanceTimersByTime(gapMs);
    await queryClient.fetchQuery(options);
    queryClient.clear();

    return queryFn.mock.calls.length;
  };

  it("serves a cached success for its staleTime", async () => {
    expect(await fetchAfter(SUCCESS, 60_000)).toBe(1);
  });

  it("serves a cached failure inside the failure window", async () => {
    expect(await fetchAfter(FAILURE, 5_000)).toBe(1);
  });

  it("refetches a failure once the window has passed", async () => {
    expect(await fetchAfter(FAILURE, 10_001)).toBe(2);
  });
});

describe("createQueryClient", () => {
  it("keeps inactive data ten minutes and layers overrides on the defaults", () => {
    const queries = createQueryClient({ queries: { retry: false } })
      .getDefaultOptions().queries;

    expect(queries?.gcTime).toBe(10 * 60_000);
    expect(queries?.retry).toBe(false);
  });

  it("gives every client its own cache", () => {
    expect(createQueryClient().getQueryCache()).not.toBe(
      createQueryClient().getQueryCache(),
    );
  });
});
