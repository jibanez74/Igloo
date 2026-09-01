// Fetch-mocking primitives shared by the route and dialog suites. Every test
// that stubs `fetch` needs the same three pieces: build a JSON `Response`,
// normalize whatever `RequestInfo` form the caller passed, and count how often
// a given URL was requested.

/** Resolves to a JSON `Response`, matching what the real API returns. */
export function jsonResponse(body: unknown, status = 200) {
  return Promise.resolve(
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

/** Normalizes the three `fetch` input forms to a plain URL string. */
export function requestURL(input: RequestInfo | URL) {
  if (typeof input === "string") return input;
  if (input instanceof URL) return input.toString();
  return input.url;
}

/** How many times a stubbed `fetch` was called with exactly `url`. */
export function countFetchRequests(
  fetchMock: { mock: { calls: [RequestInfo | URL, ...unknown[]][] } },
  url: string,
) {
  return fetchMock.mock.calls.filter(([input]) => requestURL(input) === url)
    .length;
}
