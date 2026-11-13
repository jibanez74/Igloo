import { useQueryClient } from "@tanstack/react-query";
import { fireEvent, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useSessionMutation } from "@/hooks/useSessionMutation";
import { mutationSettled, renderAcrossSessions } from "../helpers/session";

const KEY = ["probe"];

type Held = { resolve: (value: string) => void; reject: (error: Error) => void };

// A mutation that writes the cache in every callback it has, with the request
// held so the test decides when (and in which session) it lands. The probe
// state is module-level so the components below read it like any import.
const held: Held[] = [];
const calls = {
  onSuccess: vi.fn(),
  onError: vi.fn(),
  onSettled: vi.fn(),
};

function ProbePage() {
  const queryClient = useQueryClient();
  const mutation = useSessionMutation({
    mutationFn: () =>
      new Promise<string>((resolve, reject) => {
        held.push({ resolve, reject });
      }),
    onMutate: (_variables, _context, session) => ({ ended: session.ended }),
    onSuccess: (data, _variables, result) => {
      calls.onSuccess(data, result.ended());
      queryClient.setQueryData(KEY, data);
    },
    onError: (error) => {
      calls.onError(error);
      queryClient.setQueryData(KEY, "rolled back");
    },
    onSettled: () => {
      calls.onSettled();
    },
  });

  return (
    <button type="button" onClick={() => mutation.mutate()}>
      save
    </button>
  );
}

const onMutateProbe: {
  session?: { ended: () => boolean };
  release: () => void;
} = { release: () => {} };

function SlowOnMutatePage() {
  const mutation = useSessionMutation({
    mutationFn: async () => "done",
    onMutate: async (_variables, _context, session) => {
      onMutateProbe.session = session;
      await new Promise<void>((resolve) => {
        onMutateProbe.release = resolve;
      });
    },
  });

  return (
    <button type="button" onClick={() => mutation.mutate()}>
      save
    </button>
  );
}

beforeEach(() => {
  held.length = 0;
  onMutateProbe.session = undefined;
  vi.clearAllMocks();
});

describe("useSessionMutation", () => {
  it("runs the callbacks with onMutate's result while the session lasts", async () => {
    const { queryClient } = await renderAcrossSessions(ProbePage);

    fireEvent.click(screen.getByRole("button", { name: "save" }));
    await waitFor(() => expect(held).toHaveLength(1));
    held[0].resolve("saved");

    await waitFor(() => expect(calls.onSettled).toHaveBeenCalledTimes(1));
    expect(calls.onSuccess).toHaveBeenCalledWith("saved", false);
    expect(queryClient.getQueryData(KEY)).toBe("saved");
  });

  it.each([
    ["success", (request: Held) => request.resolve("previous account")],
    ["failure", (request: Held) => request.reject(new Error("boom"))],
  ])("drops a %s that lands in the next session", async (_label, land) => {
    const { queryClient, endSession, signIn } =
      await renderAcrossSessions(ProbePage);

    fireEvent.click(screen.getByRole("button", { name: "save" }));
    await waitFor(() => expect(held).toHaveLength(1));

    await endSession();
    await signIn();
    queryClient.setQueryData(KEY, "next account");

    const settled = mutationSettled(queryClient);
    land(held[0]);
    await settled;

    expect(calls.onSuccess).not.toHaveBeenCalled();
    expect(calls.onError).not.toHaveBeenCalled();
    expect(calls.onSettled).not.toHaveBeenCalled();
    expect(queryClient.getQueryData(KEY)).toBe("next account");
  });

  it("tells onMutate when the session ended during its awaits", async () => {
    const { endSession } = await renderAcrossSessions(SlowOnMutatePage);

    fireEvent.click(screen.getByRole("button", { name: "save" }));
    await waitFor(() => expect(onMutateProbe.session).toBeDefined());
    expect(onMutateProbe.session?.ended()).toBe(false);

    await endSession();
    expect(onMutateProbe.session?.ended()).toBe(true);
    onMutateProbe.release();
  });
});
