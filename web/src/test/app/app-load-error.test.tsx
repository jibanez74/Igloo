import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { AppLoadError } from "@/components/app/AppLoadingScreen";
import { ApiFailureError } from "@/lib/is-api-failure";

const router = vi.hoisted(() => ({ invalidate: vi.fn(() => Promise.resolve()) }));

vi.mock("@tanstack/react-router", async () =>
  (await import("../helpers/router-link-mock")).routerWith({
    useRouter: () => router,
  }),
);

function renderError(error: Error) {
  render(<AppLoadError error={error} reset={vi.fn()} />);
}

describe("AppLoadError", () => {
  it("blames the server for a failed API check", () => {
    renderError(new ApiFailureError({ error: true, message: "Bad gateway", status: 502 }));

    expect(screen.getByRole("alert")).toHaveTextContent("Couldn’t load Igloo.");
  });

  // Anything else is the page itself failing to render.
  it("blames the page for any other error, and Try again reloads it", async () => {
    const user = userEvent.setup();
    renderError(new TypeError("Cannot read properties of undefined"));

    expect(screen.getByRole("alert")).toHaveTextContent("Couldn’t load this page.");
    await user.click(screen.getByRole("button", { name: /try again/i }));
    expect(router.invalidate).toHaveBeenCalledOnce();
  });
});
