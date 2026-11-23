import { afterEach, describe, expect, it, vi } from "vitest";
import {
  API_REQUEST_TIMEOUT_MS,
  AVATAR_UPLOAD_TIMEOUT_MS,
  PREROLL_FETCH_TIMEOUT_MS,
} from "@/lib/constants";
import {
  getAuthUser,
  getMoviePreroll,
  getNotifications,
  login,
  logout,
  onSessionExpired,
  updateUserPassword,
  updateTrailerPreferences,
  updateUserPin,
  uploadUserAvatar,
} from "@/lib/api";
import { forgetEndedSession } from "@/lib/session";
import { deferredResponse, jsonResponse } from "../helpers/api";

const NOT_AUTHORIZED = { error: true, message: "not authorized" };

function answerEveryRequest(response: () => Promise<Response>) {
  vi.stubGlobal("fetch", vi.fn(response));
}

let unsubscribe: (() => void) | undefined;

function listenForExpiry() {
  const listener = vi.fn();
  unsubscribe = onSessionExpired(listener);
  return listener;
}

afterEach(() => {
  unsubscribe?.();
  unsubscribe = undefined;
});

describe("session expiry", () => {
  // The previous reader's request, still in flight when they signed out and
  // the next reader signed in: its 401 is about the session it was sent in.
  it.each([
    ["getNotifications", () => getNotifications()],
    ["uploadUserAvatar", () => uploadUserAvatar(new File(["x"], "me.png"))],
  ])("ignores a %s 401 that answers a session already over", async (_name, send) => {
    const answer = deferredResponse();
    answerEveryRequest(() => answer.promise);
    const listener = listenForExpiry();

    const pending = send();
    forgetEndedSession();
    answer.resolve(await jsonResponse(NOT_AUTHORIZED, 401));
    const res = await pending;

    expect(res.error).toBe(true);
    expect(listener).not.toHaveBeenCalled();
  });

  it("reports a 401 and still resolves its envelope", async () => {
    answerEveryRequest(() => jsonResponse(NOT_AUTHORIZED, 401));
    const listener = listenForExpiry();

    const res = await getNotifications();

    expect(listener).toHaveBeenCalledOnce();
    expect(res).toEqual({ ...NOT_AUTHORIZED, status: 401 });
  });

  // A proxy in front of the server can answer a 401 with an HTML page.
  it("reports a 401 whose body is not JSON", async () => {
    answerEveryRequest(() =>
      Promise.resolve(new Response("<h1>401</h1>", { status: 401 })),
    );
    const listener = listenForExpiry();

    const res = await getAuthUser();

    expect(listener).toHaveBeenCalledOnce();
    expect(res.error).toBe(true);
  });

  // Each of these answers a 401 about the request itself: wrong credentials,
  // or a logout whose session had already ended.
  it.each([
    ["login", () => login("mulder@example.com", "wrong")],
    ["logout", () => logout()],
    ["a password change", () => updateUserPassword("wrong", "NewPass12345")],
    ["a PIN change", () => updateUserPin("5678", "0000")],
  ])("leaves the session alone on a 401 from %s", async (_name, request) => {
    answerEveryRequest(() => jsonResponse(NOT_AUTHORIZED, 401));
    const listener = listenForExpiry();

    const res = await request();

    expect(listener).not.toHaveBeenCalled();
    expect(res).toMatchObject({ error: true, status: 401 });
  });

  // A first PIN sends no current one, so its 401 can only mean the session
  // ended.
  it("reports a 401 from setting a first PIN", async () => {
    answerEveryRequest(() => jsonResponse(NOT_AUTHORIZED, 401));
    const listener = listenForExpiry();

    const res = await updateUserPin("5678");

    expect(listener).toHaveBeenCalledOnce();
    expect(res).toMatchObject({ error: true, status: 401 });
  });

  it("does not report a listener's own failure as a failed upload", async () => {
    answerEveryRequest(() => jsonResponse(NOT_AUTHORIZED, 401));
    unsubscribe = onSessionExpired(() => {
      throw new Error("listener broke");
    });

    await expect(
      uploadUserAvatar(new File(["avatar"], "avatar.png", { type: "image/png" })),
    ).rejects.toThrow("listener broke");
  });

  it("sends the avatar as multipart form data, not JSON", async () => {
    const fetchMock = vi.fn(() =>
      jsonResponse({ error: false, data: { user: {} } }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const file = new File(["avatar"], "avatar.png", { type: "image/png" });

    await uploadUserAvatar(file);

    const [, init] = fetchMock.mock.calls[0] as unknown as [string, RequestInit];
    expect(init.method).toBe("POST");
    expect(init.headers).toBeUndefined();
    expect(init.body).toBeInstanceOf(FormData);
    expect((init.body as FormData).get("avatar")).toBe(file);
  });

  it("reports a 401 from the avatar upload", async () => {
    answerEveryRequest(() => jsonResponse(NOT_AUTHORIZED, 401));
    const listener = listenForExpiry();

    await uploadUserAvatar(new File(["avatar"], "avatar.png", { type: "image/png" }));

    expect(listener).toHaveBeenCalledOnce();
  });

  it("stops reporting once unsubscribed, and ignores other failures", async () => {
    answerEveryRequest(() => jsonResponse({ error: true, message: "boom" }, 500));
    const listener = listenForExpiry();

    await getNotifications();
    expect(listener).not.toHaveBeenCalled();

    unsubscribe?.();
    answerEveryRequest(() => jsonResponse(NOT_AUTHORIZED, 401));
    await getNotifications();

    expect(listener).not.toHaveBeenCalled();
  });
});

// The envelope is checked against the status: a proxy or upstream answering
// for the server (an HTML error page, a JSON body that is not the envelope)
// must never read as success, and its failure keeps the real status.
describe("response envelopes", () => {
  const htmlResponse = (status: number) =>
    Promise.resolve(
      new Response("<html><body>Bad gateway</body></html>", {
        status,
        headers: { "Content-Type": "text/html" },
      }),
    );

  it.each([
    ["non-2xx JSON without an error field", () => jsonResponse({ message: "Upstream failed" }, 502), 502],
    ["non-2xx claiming success", () => jsonResponse({ error: false, data: { user: null } }, 503), 503],
    ["an HTML error page", () => htmlResponse(504), 504],
  ])("fails %s with the real status", async (_name, answer, status) => {
    answerEveryRequest(answer);

    const res = await updateTrailerPreferences({ enabled: true, count: 2, source: "both" });

    expect(res).toEqual({
      error: true,
      message: `${status} - The request could not be completed.`,
      status,
    });
  });

  it("keeps the real status of an HTML 401", async () => {
    answerEveryRequest(() => htmlResponse(401));
    listenForExpiry();

    const res = await getNotifications();

    expect(res).toMatchObject({ error: true, status: 401 });
  });

  it("keeps the server's own failure and stamps its status", async () => {
    answerEveryRequest(() =>
      jsonResponse({ error: true, message: "trailer count must be between 1 and 5" }, 400),
    );

    const res = await updateTrailerPreferences({ enabled: true, count: 9, source: "both" });

    expect(res).toEqual({
      error: true,
      message: "trailer count must be between 1 and 5",
      status: 400,
    });
  });

  it("reports a 2xx body that is not an envelope as a network error", async () => {
    answerEveryRequest(() => jsonResponse({ enabled: true }, 200));

    const res = await updateTrailerPreferences({ enabled: true, count: 2, source: "both" });

    expect(res).toMatchObject({ error: true, status: 500 });
  });

  it.each([
    ["with data", { error: false, data: { enabled: true, count: 2, source: "both" } }],
    ["without data", { error: false, message: "Saved" }],
  ])("passes a success envelope %s through", async (_name, body) => {
    answerEveryRequest(() => jsonResponse(body, 200));

    const res = await updateTrailerPreferences({ enabled: true, count: 2, source: "both" });

    expect(res).toEqual(body);
  });

  it("fails an avatar upload whose non-2xx body is not an envelope", async () => {
    answerEveryRequest(() => jsonResponse({ detail: "too large" }, 413));

    const res = await uploadUserAvatar(new File(["x"], "me.png", { type: "image/png" }));

    expect(res).toEqual({
      error: true,
      message: "413 - The avatar could not be uploaded.",
      status: 413,
    });
  });
});

describe("request deadline", () => {
  const TIMED_OUT = {
    error: true,
    message: "504 - The server took too long to respond.",
    status: 504,
  };

  // A server that accepts the request and never answers: fetch rejects only
  // when its signal aborts, as the real one does.
  function neverAnswer() {
    const fetchMock = vi.fn(
      (_input: RequestInfo | URL, init?: RequestInit) =>
        new Promise<Response>((_resolve, reject) => {
          if (init?.signal?.aborted) reject(init.signal.reason);
          init?.signal?.addEventListener("abort", () =>
            reject(init.signal?.reason),
          );
        }),
    );
    vi.stubGlobal("fetch", fetchMock);
    return fetchMock;
  }

  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("resolves a timeout envelope once a request goes unanswered too long", async () => {
    vi.useFakeTimers();
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    neverAnswer();
    let settled = false;
    const pending = getNotifications().then((res) => {
      settled = true;
      return res;
    });

    await vi.advanceTimersByTimeAsync(API_REQUEST_TIMEOUT_MS - 1);
    expect(settled).toBe(false);
    await vi.advanceTimersByTimeAsync(1);

    await expect(pending).resolves.toEqual(TIMED_OUT);
    expect(warn).toHaveBeenCalledOnce();
  });

  it("bounds a body that stops arriving too", async () => {
    vi.useFakeTimers();
    vi.spyOn(console, "warn").mockImplementation(() => {});
    vi.stubGlobal(
      "fetch",
      vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
        const body = new ReadableStream<Uint8Array>({
          start(stream) {
            stream.enqueue(new TextEncoder().encode('{"error":'));
            init?.signal?.addEventListener("abort", () =>
              stream.error(init.signal?.reason),
            );
          },
        });
        return Promise.resolve(new Response(body, { status: 200 }));
      }),
    );

    const pending = getNotifications();
    await vi.advanceTimersByTimeAsync(API_REQUEST_TIMEOUT_MS);

    await expect(pending).resolves.toEqual(TIMED_OUT);
  });

  it.each([
    ["the preroll queue", () => getMoviePreroll(7), PREROLL_FETCH_TIMEOUT_MS],
    [
      "an avatar upload",
      () => uploadUserAvatar(new File(["x"], "me.png")),
      AVATAR_UPLOAD_TIMEOUT_MS,
    ],
  ])("gives %s its own deadline", async (_name, send, deadline) => {
    vi.useFakeTimers();
    vi.spyOn(console, "warn").mockImplementation(() => {});
    neverAnswer();
    let settled = false;
    const pending = send().then((res) => {
      settled = true;
      return res;
    });

    await vi.advanceTimersByTimeAsync(deadline - 1);
    expect(settled).toBe(false);
    await vi.advanceTimersByTimeAsync(1);

    await expect(pending).resolves.toEqual(TIMED_OUT);
  });

  it("still answers a caller's abort with the silent network envelope", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    neverAnswer();
    const caller = new AbortController();

    const pending = getMoviePreroll(7, caller.signal);
    caller.abort();

    await expect(pending).resolves.toMatchObject({ error: true, status: 500 });
    expect(warn).not.toHaveBeenCalled();
  });

  // AbortSignal.timeout aborts with a TimeoutError too, but that is the
  // caller's own bound (the watch-progress saves), not the helper's deadline.
  it("treats a caller's TimeoutError abort as the caller giving up", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    neverAnswer();
    const caller = new AbortController();

    const pending = getMoviePreroll(7, caller.signal);
    caller.abort(new DOMException("caller bound", "TimeoutError"));

    await expect(pending).resolves.toMatchObject({ error: true, status: 500 });
    expect(warn).not.toHaveBeenCalled();
  });

  it("stays quiet when the caller aborts while the body is read", async () => {
    const warn = vi.spyOn(console, "warn").mockImplementation(() => {});
    let started!: () => void;
    const bodyStarted = new Promise<void>((resolve) => (started = resolve));
    vi.stubGlobal(
      "fetch",
      vi.fn((_input: RequestInfo | URL, init?: RequestInit) => {
        const body = new ReadableStream<Uint8Array>({
          start(stream) {
            stream.enqueue(new TextEncoder().encode('{"error":'));
            init?.signal?.addEventListener("abort", () =>
              stream.error(new Error("custom reason")),
            );
            started();
          },
        });
        return Promise.resolve(new Response(body, { status: 200 }));
      }),
    );
    const caller = new AbortController();

    const pending = getMoviePreroll(7, caller.signal);
    await bodyStarted;
    caller.abort();

    await expect(pending).resolves.toMatchObject({ error: true, status: 500 });
    expect(warn).not.toHaveBeenCalled();
  });

  it("aborts at once for a signal already aborted", async () => {
    const fetchMock = neverAnswer();
    const caller = new AbortController();
    caller.abort();

    await expect(getMoviePreroll(7, caller.signal)).resolves.toMatchObject({
      status: 500,
    });
    expect(fetchMock.mock.calls[0][1]?.signal?.aborted).toBe(true);
  });

  it("clears its deadline and caller listener once the request settles", async () => {
    vi.useFakeTimers();
    answerEveryRequest(() => jsonResponse({ error: false, data: {} }));
    const caller = new AbortController();
    const remove = vi.spyOn(caller.signal, "removeEventListener");

    await expect(getMoviePreroll(7, caller.signal)).resolves.toMatchObject({
      error: false,
    });

    expect(vi.getTimerCount()).toBe(0);
    expect(remove).toHaveBeenCalledWith("abort", expect.any(Function));
  });
});
