import { screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import MediaDetailGuard from "@/components/shared/MediaDetailGuard";
import { renderWithQueryClient } from "../helpers/render";

vi.mock("@tanstack/react-router", async () =>
  (await import("../helpers/router-link-mock")).routerWithAnchorLinks(),
);

type Payload = { name: string };

function renderGuard(
  overrides: Partial<React.ComponentProps<typeof MediaDetailGuard<Payload>>>,
) {
  return renderWithQueryClient(
    <MediaDetailGuard<Payload>
      id={7}
      noun="album"
      back="music"
      isPending={false}
      isError={false}
      data={{ error: false }}
      payload={{ name: "Blue Record" }}
      skeleton={<div data-testid="skeleton" />}
      {...overrides}
    >
      {(loaded, id) => (
        <p>
          {loaded.name} #{id}
        </p>
      )}
    </MediaDetailGuard>,
  );
}

describe("MediaDetailGuard", () => {
  it("renders the subject once everything has arrived", () => {
    renderGuard({});

    expect(screen.getByText("Blue Record #7")).toBeInTheDocument();
  });

  it("rejects a link that never named a subject", () => {
    renderGuard({ id: null });

    expect(screen.getByRole("alert")).toHaveTextContent(
      "That album link is not valid.",
    );
  });

  it("prefers the server's message when the request failed", () => {
    renderGuard({
      isError: true,
      data: { error: true, message: "The library is rescanning." },
      payload: null,
    });

    expect(screen.getByRole("alert")).toHaveTextContent(
      "The library is rescanning.",
    );
  });

  it("falls back to its own wording when the failure carried no message", () => {
    renderGuard({ isError: true, data: undefined, payload: null });

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Failed to load album details. Please try again later.",
    );
  });

  it("shows the caller's skeleton while the request is in flight", () => {
    renderGuard({ isPending: true, payload: null });

    expect(screen.getByTestId("skeleton")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("reports a response that came back without the subject", () => {
    renderGuard({ payload: null });

    expect(screen.getByRole("alert")).toHaveTextContent("Album not found.");
  });

  // Every failing branch is a dead end, so each must offer a way out.
  it("gives every failure a way back", () => {
    for (const props of [
      { id: null },
      { isError: true, payload: null },
      { payload: null },
    ]) {
      const { unmount } = renderGuard(props);

      expect(
        screen.getByRole("link", { name: "Back to Music" }),
      ).toHaveAttribute("href", "/music");

      unmount();
    }
  });
});
