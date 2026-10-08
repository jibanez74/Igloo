import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Button } from "@/components/ui/button";

// A disabled primary button used to be the same glacier at half opacity, which
// still read as a live action next to a real one (design-system §1.6).
describe("Button disabled styling", () => {
  it.each(["default", "accent", "accent-pill"] as const)(
    "swaps the %s fill for the muted surface when disabled",
    variant => {
      render(
        <Button variant={variant} disabled>
          Send Request
        </Button>,
      );

      const button = screen.getByRole("button", { name: "Send Request" });
      expect(button).toBeDisabled();
      expect(button).toHaveClass(
        "disabled:bg-muted",
        "disabled:text-muted-foreground",
        "disabled:opacity-100",
      );
      // The base opacity is overridden, not stacked on top of the muted fill.
      expect(button).not.toHaveClass("disabled:opacity-50");
    },
  );

  it("keeps the base half-opacity for outline and ghost buttons", () => {
    render(
      <>
        <Button variant="outline" disabled>
          Reset
        </Button>
        <Button variant="ghost" disabled>
          Dismiss
        </Button>
      </>,
    );

    for (const name of ["Reset", "Dismiss"]) {
      const button = screen.getByRole("button", { name });
      expect(button).toHaveClass("disabled:opacity-50");
      expect(button).not.toHaveClass("disabled:bg-muted");
    }
  });
});
