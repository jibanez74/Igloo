import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { jsonResponse, requestURL } from "../helpers/api";
import { authUser } from "../helpers/fixtures";
import { readDocumentHead, renderRoute } from "../helpers/render-route";

function mockShellFetch() {
  vi.stubGlobal(
    "fetch",
    vi.fn((input: RequestInfo | URL) => {
      if (requestURL(input) === "/api/auth/user") {
        return jsonResponse(authUser());
      }

      return jsonResponse({ error: false, data: {} });
    }),
  );
}

// The router's default not-found view was one unstyled line with no shell
// and no way back; unknown URLs now land on a page inside the shell.
describe("unknown URLs", () => {
  it("render a not-found page inside the authenticated shell", async () => {
    mockShellFetch();

    await renderRoute("/does-not-exist");

    expect(
      await screen.findByRole("heading", { level: 1, name: "Page not found" }),
    ).toBeInTheDocument();
    // The heading names the failure; the alert adds one sentence, untitled.
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("There is no page at this address.");
    expect(alert).not.toHaveTextContent("Not found");
    expect(screen.getByRole("link", { name: "Back to Home" })).toHaveAttribute(
      "href",
      "/",
    );
    expect(screen.getByRole("link", { name: "Movies" })).toBeInTheDocument();
    expect(readDocumentHead().title).toBe("Page not found - Igloo");
  });

  it("catch nested paths under a known section too", async () => {
    mockShellFetch();

    await renderRoute("/movies/7/nope");

    expect(
      await screen.findByRole("heading", { level: 1, name: "Page not found" }),
    ).toBeInTheDocument();
  });
});
