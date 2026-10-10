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

    expect(
      screen.getByRole("heading", { level: 1, name: "Album not found" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "That album link is not valid.",
    );
  });

  it("words a 403 as the reader lacking access, not the server's constant", () => {
    renderGuard({
      isError: false,
      data: { error: true, message: "access denied", status: 403 },
      payload: null,
    });

    expect(
      screen.getByRole("heading", { level: 1, name: "No access" }),
    ).toBeInTheDocument();
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("You don't have access to this album.");
    expect(alert).not.toHaveTextContent("access denied");
  });

  // Whatever the server (or the client's network envelope) said, the reader
  // gets the guard's own sentence: a lowercase constant and the canned
  // "500 - A network error…" string are not copy.
  it.each([
    [500, "500 - A network error occurred while processing your request."],
    [503, "The library is rescanning."],
    [401, "not authorized"],
  ])("shows its own sentence for a %i failure", (status, message) => {
    renderGuard({
      isError: true,
      data: { error: true, message, status },
      payload: null,
    });

    expect(
      screen.getByRole("heading", { level: 1, name: "Couldn’t load this album" }),
    ).toBeInTheDocument();
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(
      "Something went wrong while loading this album. Please try again later.",
    );
    expect(alert).not.toHaveTextContent(message);
  });

  // apiRequest answers every 404 with "404 - The resource you requested was
  // not found."; the reader should see a sentence about their subject instead.
  it("words a 404 as the subject being missing, not the client's status string", () => {
    renderGuard({
      isError: false,
      data: {
        error: true,
        message: "404 - The resource you requested was not found.",
        status: 404,
      },
      payload: null,
    });

    expect(
      screen.getByRole("heading", { level: 1, name: "Album not found" }),
    ).toBeInTheDocument();
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("We couldn't find that album.");
    expect(alert).not.toHaveTextContent("404");
  });

  it("uses the same wording when the failure carried no envelope at all", () => {
    renderGuard({ isError: true, data: undefined, payload: null });

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Something went wrong while loading this album. Please try again later.",
    );
  });

  it("shows the caller's skeleton while the request is in flight", () => {
    renderGuard({ isPending: true, payload: null });

    expect(screen.getByTestId("skeleton")).toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("reports a response that came back without the subject", () => {
    renderGuard({ payload: null });

    expect(
      screen.getByRole("heading", { level: 1, name: "Album not found" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent(
      "We couldn't find that album.",
    );
  });

  // Every failing branch is a dead end, so each must offer a way out.
  it("gives every failure a way back", () => {
    for (const props of [
      { id: null },
      { isError: true, payload: null },
      { data: { error: true, status: 403 }, payload: null },
      { data: { error: true, status: 404 }, payload: null },
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
