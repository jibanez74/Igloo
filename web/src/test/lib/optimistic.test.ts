import { queryOptions } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import { restoreOptimistic, writeOptimistically } from "@/lib/optimistic";
import type { ApiResponseType } from "@/types";
import { createTestQueryClient } from "../helpers/render";

type Liked = ApiResponseType<{ is_liked: boolean }>;

const liked = (is_liked: boolean): Liked => ({ error: false, data: { is_liked } });
const likeOpts = queryOptions({
  queryKey: ["optimistic-test"],
  queryFn: () => Promise.resolve(liked(false)),
});
const live = { ended: () => false };

describe("writeOptimistically", () => {
  it("writes the next value and hands back what was cached", async () => {
    const queryClient = createTestQueryClient();
    queryClient.setQueryData(likeOpts.queryKey, liked(false));

    const snapshot = await writeOptimistically(
      queryClient,
      likeOpts.queryKey,
      live,
      previous => liked(previous?.error === false ? !previous.data.is_liked : true),
    );

    expect(queryClient.getQueryData(likeOpts.queryKey)).toEqual(liked(true));
    expect(snapshot).toEqual({ previous: liked(false) });
  });

  it("cancels a fetch in flight so it cannot land over the write", async () => {
    const queryClient = createTestQueryClient();
    const cancel = vi.spyOn(queryClient, "cancelQueries");

    await writeOptimistically(queryClient, likeOpts.queryKey, live, () => liked(true));

    expect(cancel).toHaveBeenCalledWith({ queryKey: likeOpts.queryKey });
  });

  it("writes nothing once the session has ended", async () => {
    const queryClient = createTestQueryClient();
    queryClient.setQueryData(likeOpts.queryKey, liked(false));

    const snapshot = await writeOptimistically(
      queryClient,
      likeOpts.queryKey,
      { ended: () => true },
      () => liked(true),
    );

    expect(snapshot).toBeUndefined();
    expect(queryClient.getQueryData(likeOpts.queryKey)).toEqual(liked(false));
  });

  it("leaves the entry alone when there is nothing to edit", async () => {
    const queryClient = createTestQueryClient();

    const snapshot = await writeOptimistically(
      queryClient,
      likeOpts.queryKey,
      live,
      () => undefined,
    );

    expect(snapshot).toEqual({ previous: undefined });
    expect(queryClient.getQueryData(likeOpts.queryKey)).toBeUndefined();
  });
});

describe("restoreOptimistic", () => {
  it("puts back the value cached before the write", () => {
    const queryClient = createTestQueryClient();
    queryClient.setQueryData(likeOpts.queryKey, liked(true));

    restoreOptimistic(queryClient, likeOpts.queryKey, { previous: liked(false) });

    expect(queryClient.getQueryData(likeOpts.queryKey)).toEqual(liked(false));
  });

  it("refetches rather than keep a guess when nothing was cached before", () => {
    const queryClient = createTestQueryClient();
    queryClient.setQueryData(likeOpts.queryKey, liked(true));

    restoreOptimistic(queryClient, likeOpts.queryKey, { previous: undefined });

    expect(queryClient.getQueryState(likeOpts.queryKey)?.isInvalidated).toBe(true);
  });

  it("does nothing for a write that never happened", () => {
    const queryClient = createTestQueryClient();
    queryClient.setQueryData(likeOpts.queryKey, liked(true));

    restoreOptimistic(queryClient, likeOpts.queryKey, undefined);

    expect(queryClient.getQueryData(likeOpts.queryKey)).toEqual(liked(true));
    expect(queryClient.getQueryState(likeOpts.queryKey)?.isInvalidated).toBe(false);
  });
});
