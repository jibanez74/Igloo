import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Button, buttonVariants } from "@/components/ui/button";

// A disabled primary button used to be the same glacier at half opacity, which
// still read as a live action next to a real one (design-system §1.6).
describe("Button disabled styling", () => {
  it.each(["default", "accent"] as const)(
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

// Sizes carry no radius of their own: the base rounds every button and the
// pill shape alone overrides it.
describe("Button pill shape", () => {
  it.each(["default", "sm", "lg"] as const)("stays rounded-full at size %s", size => {
    render(
      <Button variant="accent" shape="pill" size={size}>
        Play
      </Button>,
    );

    const button = screen.getByRole("button", { name: "Play" });
    expect(button).toHaveAttribute("data-shape", "pill");
    expect(button).toHaveClass("rounded-full");
    expect(button).not.toHaveClass("rounded-md");
  });

  it.each(["accent", "outline", "secondary", "ghost"] as const)(
    "applies to the %s variant without changing its colours",
    variant => {
      render(
        <>
          <Button variant={variant} shape="pill">
            Pill
          </Button>
          <Button variant={variant}>Square</Button>
        </>,
      );

      const pill = screen.getByRole("button", { name: "Pill" });
      const square = screen.getByRole("button", { name: "Square" });
      const withoutRadius = (button: HTMLElement) =>
        button.className
          .split(" ")
          .filter(name => !name.startsWith("rounded-"))
          .sort();
      expect(withoutRadius(pill)).toEqual(withoutRadius(square));
      expect(square).toHaveClass("rounded-md");
    },
  );
});

// The looks call sites used to assemble from an outline or ghost button and a
// handful of classes (design-system §1.6).
describe("Button quiet variants", () => {
  it("fills the secondary action with the muted surface and text", () => {
    render(<Button variant="secondary">Cancel</Button>);

    const button = screen.getByRole("button", { name: "Cancel" });
    expect(button).toHaveClass(
      "border-border",
      "bg-muted",
      "text-muted-foreground",
      "hover:bg-accent",
      "hover:text-foreground",
    );
    expect(button).not.toHaveClass("bg-background", "text-foreground");
  });

  it("rests a ghost-muted button in the muted text colour", () => {
    render(
      <Button variant="ghost-muted" size="icon" aria-label="Toggle theme" />,
    );

    const button = screen.getByRole("button", { name: "Toggle theme" });
    expect(button).toHaveClass(
      "text-muted-foreground",
      "hover:bg-accent",
      "hover:text-foreground",
    );
    expect(button).not.toHaveClass("border", "bg-muted");
  });

  it("warns on hover for a ghost-destructive button", () => {
    render(
      <Button variant="ghost-destructive" size="icon" aria-label="Delete" />,
    );

    const button = screen.getByRole("button", { name: "Delete" });
    expect(button).toHaveClass("text-muted-foreground", "hover:text-destructive");
    expect(button).not.toHaveClass("hover:text-foreground", "text-destructive");
  });

  it("outlines a destructive action in its colour", () => {
    render(<Button variant="outline-destructive">Revoke</Button>);

    const button = screen.getByRole("button", { name: "Revoke" });
    expect(button).toHaveClass(
      "border-destructive/50",
      "text-destructive",
      "hover:bg-destructive/10",
    );
    expect(button).not.toHaveClass("bg-destructive");
  });
});

// buttonVariants styles elements that are not a Button (links, Radix
// triggers), often with no cn() around it, so the string it returns must
// already be merged: one class per property, the later source winning.
describe("buttonVariants", () => {
  const classesOf = (value: string) => value.split(" ");

  it("defaults to the default variant at the default size", () => {
    expect(buttonVariants()).toBe(
      buttonVariants({ variant: "default", size: "default" }),
    );
    expect(classesOf(buttonVariants())).toEqual(
      expect.arrayContaining(["bg-primary", "h-9", "rounded-md"]),
    );
  });

  it("lets the size's gap replace the base gap", () => {
    const classes = classesOf(buttonVariants({ variant: "outline", size: "sm" }));

    expect(classes).toContain("gap-1.5");
    expect(classes).not.toContain("gap-2");
  });

  it("keeps the pill round at every size", () => {
    for (const size of ["default", "sm", "lg"] as const) {
      const classes = classesOf(
        buttonVariants({ variant: "accent", shape: "pill", size }),
      );

      expect(classes).toContain("rounded-full");
      expect(classes).not.toContain("rounded-md");
    }
  });

  it("lets className win over the variant and the size", () => {
    const classes = classesOf(
      buttonVariants({ variant: "outline", size: "lg", className: "h-12 bg-card" }),
    );

    expect(classes).toEqual(expect.arrayContaining(["h-12", "bg-card"]));
    expect(classes).not.toContain("h-10");
    expect(classes).not.toContain("bg-background");
  });

  it("never repeats a class", () => {
    for (const variant of [
      "default",
      "destructive",
      "outline",
      "ghost",
      "accent",
      "secondary",
      "outline-destructive",
      "ghost-muted",
      "ghost-destructive",
    ] as const) {
      for (const size of ["default", "sm", "lg", "icon", "icon-sm"] as const) {
        const classes = classesOf(buttonVariants({ variant, size }));

        expect(new Set(classes).size).toBe(classes.length);
      }
    }
  });
});
