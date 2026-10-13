import { afterEach, describe, expect, it, vi } from "vitest";
import {
  getAuthUser,
  getNotifications,
  login,
  logout,
  onSessionExpired,
  updateUserPassword,
  updateUserPin,
  uploadUserAvatar,
} from "@/lib/api";
import { jsonResponse } from "../helpers/api";

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
