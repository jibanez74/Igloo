import { beforeEach, describe, expect, it, vi } from "vitest";
import { watchRoomQueryOpts } from "@/lib/query-opts";
import { createTestQueryClient } from "../helpers/render";

const getWatchRoomMock = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api", async () => ({
  ...(await vi.importActual<typeof import("@/lib/api")>("@/lib/api")),
  getWatchRoom: (...args: unknown[]) => getWatchRoomMock(...args),
}));

const room = { error: false as const, data: { room: { id: 7, movie_title: "Arrival" } } };

async function refetchWith(cached: unknown, answer: unknown) {
  const client = createTestQueryClient();
  const opts = watchRoomQueryOpts(7);
  client.setQueryData(opts.queryKey, cached as never);
  getWatchRoomMock.mockResolvedValueOnce(answer);
  const result = await client.fetchQuery({ ...opts, staleTime: 0 });
  expect(getWatchRoomMock).toHaveBeenCalled();
  return result;
}

beforeEach(() => {
  getWatchRoomMock.mockReset();
});

describe("watch room query", () => {
  // Cached as data, a failed refetch tore down the room in progress, and the
  // members updates written into the cache had no room to land in.
  it("keeps the loaded room through a refetch that fails for a moment", async () => {
    const failure = { error: true, message: "Network error", status: 500 };
    expect(await refetchWith(room, failure)).toEqual(room);
  });

  it("lets the server's 404 or 403 replace the room", async () => {
    const gone = { error: true, message: "not found", status: 404 };
    expect(await refetchWith(room, gone)).toEqual(gone);
    const refused = { error: true, message: "access denied", status: 403 };
    expect(await refetchWith(room, refused)).toEqual(refused);
  });

  it("does not bring back a room after it was gone", async () => {
    const refused = { error: true, message: "access denied", status: 403 };
    const failure = { error: true, message: "Network error", status: 500 };
    expect(await refetchWith(refused, failure)).toEqual(failure);
  });
});
