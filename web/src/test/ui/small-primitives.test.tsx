import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Badge } from "@/components/ui/badge";
import { Card, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { expectNoClass } from "../helpers/dom";

// Spinner, Label, Card and Badge take what call sites used to pass as
// classes: a size, a tone, a surface, a variant (design-system §1.6).

describe("Spinner", () => {
  it("is small and takes the surrounding text colour by default", () => {
    render(<Spinner />);

    const spinner = screen.getByRole("status", { name: "Loading" });
    expect(spinner).toHaveClass("size-4");
    expect(spinner.getAttribute("class")).not.toMatch(/text-(primary|success)/);
  });

  it.each([
    ["sm", "size-4"],
    ["md", "size-5"],
    ["lg", "size-6"],
    ["xl", "size-8"],
    ["2xl", "size-10"],
  ] as const)("is %s at %s", (size, className) => {
    render(<Spinner size={size} />);

    const spinner = screen.getByRole("status", { name: "Loading" });
    expect(spinner).toHaveClass(className);
    for (const other of ["size-4", "size-5", "size-6", "size-8", "size-10"]) {
      if (other !== className) expect(spinner).not.toHaveClass(other);
    }
  });

  // Inside an outline button every icon is forced to the button's colour;
  // the tone has to outrank that to show at all.
  it.each([
    ["primary", "text-primary!"],
    ["success", "text-success!"],
  ] as const)("keeps its %s tone against a forced icon colour", (tone, cls) => {
    render(<Spinner tone={tone} />);

    expect(screen.getByRole("status", { name: "Loading" })).toHaveClass(cls);
  });

  it("stops spinning under reduced motion", () => {
    render(<Spinner />);

    expect(screen.getByRole("status", { name: "Loading" })).toHaveClass(
      "animate-spin",
      "motion-reduce:animate-none",
    );
  });
});

describe("Label tone", () => {
  it("is the foreground colour by default and muted when asked", () => {
    render(
      <>
        <Label htmlFor="a">Name</Label>
        <Label htmlFor="b" tone="muted">
          Download speed
        </Label>
      </>,
    );

    expect(screen.getByText("Name")).toHaveClass("text-foreground");
    const muted = screen.getByText("Download speed");
    expect(muted).toHaveClass("text-muted-foreground");
    expect(muted).not.toHaveClass("text-foreground");
  });
});

describe("Card surface", () => {
  const surfaceOf = (surface?: React.ComponentProps<typeof Card>["surface"]) => {
    render(<Card surface={surface} data-testid="card" />);
    return screen.getByTestId("card");
  };

  it("is the muted Settings panel by default", () => {
    const card = surfaceOf();

    expect(card).toHaveAttribute("data-surface", "muted");
    expect(card).toHaveClass("border-border/50", "bg-muted/30");
    expect(card).not.toHaveClass("bg-card");
  });

  it("is the opaque card when raised", () => {
    const card = surfaceOf("raised");

    expect(card).toHaveClass("bg-card");
    expectNoClass(card, "bg-muted/30", "border-border/50");
  });

  it.each([
    ["error", "border-destructive/20"],
    ["danger", "border-destructive/50"],
  ] as const)("tints the %s surface destructive", (surface, border) => {
    const card = surfaceOf(surface);

    expect(card).toHaveClass(border, "bg-destructive/10");
    expectNoClass(card, "bg-muted/30", "bg-card");
  });

  it("gives its title the destructive tone when asked", () => {
    render(
      <>
        <CardTitle>Devices</CardTitle>
        <CardTitle tone="destructive">Danger Zone</CardTitle>
      </>,
    );

    expect(screen.getByText("Devices")).not.toHaveClass("text-destructive");
    expect(screen.getByText("Danger Zone")).toHaveClass("text-destructive");
  });
});

describe("Badge", () => {
  it.each([
    ["default", ["bg-primary", "text-primary-foreground"]],
    ["outline", ["border-border", "bg-background/60", "text-muted-foreground"]],
    ["over-media", ["border-white/25", "bg-black/30", "text-white/90"]],
    ["muted", ["border-border/40", "bg-muted/90", "text-foreground"]],
    ["tinted", ["border-primary/30", "bg-muted/80", "text-primary"]],
  ] as const)("draws the %s variant", (variant, classes) => {
    render(<Badge variant={variant}>PG-13</Badge>);

    const badge = screen.getByText("PG-13");
    expect(badge).toHaveAttribute("data-variant", variant);
    expect(badge).toHaveClass(...classes);
  });

  it("pads and sizes its text by size", () => {
    render(
      <>
        <Badge size="sm">7.6</Badge>
        <Badge>Owner</Badge>
        <Badge size="lg">HD</Badge>
      </>,
    );

    expect(screen.getByText("7.6")).toHaveClass("px-2", "py-0.5", "text-xs");
    expect(screen.getByText("Owner")).toHaveClass("px-2.5", "py-0.5", "text-xs");
    const large = screen.getByText("HD");
    expect(large).toHaveClass("px-3", "py-1", "text-sm");
    expectNoClass(large, "px-2.5", "py-0.5", "text-xs");
  });
});
