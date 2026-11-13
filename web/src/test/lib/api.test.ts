import { afterEach, describe, expect, it, vi } from "vitest";
import {
  getAuthUser,
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
