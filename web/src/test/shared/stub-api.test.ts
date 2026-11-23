import { describe, expect, it } from "vitest";
import { jsonResponse, stubApi } from "../helpers/api";

async function body(response: Response) {
  return response.json() as Promise<unknown>;
}

describe("stubApi", () => {
  it("answers values as data envelopes and passes Responses through", async () => {
    stubApi({
      "/api/movies/latest": { movies: [] },
      "POST /api/movies/7/like": () => jsonResponse({ error: true, message: "no" }, 500),
    });

    expect(await body(await fetch("/api/movies/latest"))).toEqual({
      error: false,
      data: { movies: [] },
    });
    const failed = await fetch("/api/movies/7/like", { method: "POST" });
    expect(failed.status).toBe(500);
  });

  it("answers the signed-in user unless told not to", async () => {
    stubApi({}, { user: { name: "Ada" } });

    const answer = (await body(await fetch("/api/auth/user"))) as {
      data: { user: { name: string } };
    };
    expect(answer.data.user.name).toBe("Ada");
  });

  it("prefers the route with the query, then the method", async () => {
    const seen: string[] = [];
    stubApi({
      "/api/tracks?page=2": "second page",
      "/api/tracks": ({ search, method }) => {
        seen.push(`${method} page=${search.get("page")}`);
        return "any page";
      },
      "DELETE /api/tracks": "deleted",
    });

    expect(await body(await fetch("/api/tracks?page=2"))).toMatchObject({ data: "second page" });
    expect(await body(await fetch("/api/tracks?page=3"))).toMatchObject({ data: "any page" });
    expect(await body(await fetch("/api/tracks", { method: "DELETE" }))).toMatchObject({
      data: "deleted",
    });
    expect(seen).toEqual(["GET page=3"]);
  });

  it("resolves an async route's plain data into the envelope", async () => {
    stubApi({ "/api/slow": async () => ({ ready: true }) }, { user: null });

    expect(await body(await fetch("/api/slow"))).toEqual({
      error: false,
      data: { ready: true },
    });
  });

  it("answers a request no route answers with a 404 and reports it", async () => {
    const reported: string[] = [];
    stubApi({}, { user: null, onUnhandled: request => reported.push(request) });

    const answer = await fetch("/api/nowhere?page=2", { method: "POST" });

    expect(answer.status).toBe(404);
    expect(await body(answer)).toEqual({
      error: true,
      message: "Unhandled stubApi request",
    });
    expect(reported).toEqual(["POST /api/nowhere?page=2"]);
  });

  // Without onUnhandled the same request fails the test when it finishes;
  // the body does nothing else that could fail.
  it.fails("fails the test on a request no route answers", async () => {
    stubApi({}, { user: null });

    await fetch("/api/nowhere");
  });
});
