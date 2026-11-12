import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { FieldError, FieldHint } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

// A field's help and its error are one size and one colour each, wherever the
// field is (design-system §3.7).

describe("FieldHint", () => {
  it("is quiet help text that a control can point at", () => {
    render(
      <>
        <Label htmlFor="speed">Download speed (Mbps)</Label>
        <FieldHint id="speed-hint">Leave blank if unsure.</FieldHint>
        <Input id="speed" aria-describedby="speed-hint" />
      </>,
    );

    const hint = screen.getByText("Leave blank if unsure.");
    expect(hint.tagName).toBe("P");
    expect(hint).toHaveAttribute("data-slot", "field-hint");
    expect(hint).toHaveClass("text-sm", "text-muted-foreground");
    expect(hint).not.toHaveAttribute("role");
    expect(
      screen.getByRole("textbox", { name: "Download speed (Mbps)" }),
    ).toHaveAccessibleDescription("Leave blank if unsure.");
  });

  it("keeps a layout class the caller adds", () => {
    render(<FieldHint className="mt-2">Help</FieldHint>);

    expect(screen.getByText("Help")).toHaveClass("mt-2", "text-sm");
  });
});

describe("FieldError", () => {
  it("is announced when it appears and reads as an error", () => {
    render(<FieldError id="name-error">Name is required.</FieldError>);

    const error = screen.getByRole("alert");
    expect(error).toHaveTextContent("Name is required.");
    expect(error).toHaveAttribute("id", "name-error");
    expect(error).toHaveAttribute("data-slot", "field-error");
    expect(error).toHaveClass("text-sm", "text-destructive");
  });
});
