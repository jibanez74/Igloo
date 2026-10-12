import { afterEach, describe, expect, it, vi } from "vitest";
import { focusManager, useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import {
  likedTrackIdsQueryOpts,
  tracksInfiniteQueryOpts,
} from "@/lib/query-opts";
import { jsonResponse } from "../helpers/api";
import { createTestQueryClient, queryClientWrapper } from "../helpers/render";

const TRACKS_PAGE = {
  error: false,
  data: { tracks: [], total: 0, offset: 0, limit: 50, has_more: false },
};

const LIKED_IDS = { error: false, data: { liked_track_ids: [] } };

const refocusWindow = async () => {
  act(() => {
    focusManager.setFocused(false);
    focusManager.setFocused(true);
  });
  await new Promise(resolve => setTimeout(resolve, 20));
};

describe("window-focus refetching", () => {
  afterEach(() => {
    focusManager.setFocused(undefined);
    vi.unstubAllGlobals();
  });

  it("skips it for an infinite track list, which would replay every loaded page", async () => {
    const fetchMock = vi.fn(() => jsonResponse(TRACKS_PAGE));
    vi.stubGlobal("fetch", fetchMock);
    const queryClient = createTestQueryClient();
    const options = tracksInfiniteQueryOpts();

    const { result } = renderHook(() => useInfiniteQuery(options), {
      wrapper: queryClientWrapper(queryClient),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    await queryClient.invalidateQueries({
      queryKey: options.queryKey,
      refetchType: "none",
    });
    await refocusWindow();

    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("keeps it for an ordinary stale query", async () => {
    const fetchMock = vi.fn(() => jsonResponse(LIKED_IDS));
    vi.stubGlobal("fetch", fetchMock);
    const queryClient = createTestQueryClient();
    const options = likedTrackIdsQueryOpts();

    const { result } = renderHook(() => useQuery(options), {
      wrapper: queryClientWrapper(queryClient),
    });
    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    await queryClient.invalidateQueries({
      queryKey: options.queryKey,
      refetchType: "none",
    });
    await refocusWindow();

    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
  });
});
