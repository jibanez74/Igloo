import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { stubApi } from "../helpers/api";
import { readDocumentHead, renderRoute } from "../helpers/render-route";

// The router's default not-found view was one unstyled line with no shell
// and no way back; unknown URLs now land on a page inside the shell.
describe("unknown URLs", () => {
  it.each(["/does-not-exist", "/photos", "/photos/"])(
    "renders %s as not found inside the authenticated shell",
    async path => {
      // The shell needs only the signed-in user.
      stubApi({});

      await renderRoute(path);

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
      expect(screen.queryByRole("link", { name: "Photos" })).not.toBeInTheDocument();
      expect(readDocumentHead().title).toBe("Page not found - Igloo");
    },
  );

  it("catch nested paths under a known section too", async () => {
    stubApi({});

    await renderRoute("/movies/7/nope");

    expect(
      await screen.findByRole("heading", { level: 1, name: "Page not found" }),
    ).toBeInTheDocument();
  });
});
