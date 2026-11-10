import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { RadioCard } from "@/components/ui/radio-card";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import { expectNoClass } from "../helpers/dom";

describe("Switch", () => {
  it("is a switch that reports the opposite of its state when clicked", () => {
    const onCheckedChange = vi.fn();
    const { rerender } = render(
      <Switch checked={false} onCheckedChange={onCheckedChange} aria-label="Autoplay" />,
    );

    const toggle = screen.getByRole("switch", { name: "Autoplay" });
    expect(toggle).toHaveAttribute("aria-checked", "false");
    expect(toggle).toHaveAttribute("data-state", "unchecked");
    expect(toggle).toHaveAttribute("type", "button");

    fireEvent.click(toggle);
    expect(onCheckedChange).toHaveBeenLastCalledWith(true);

    rerender(<Switch checked onCheckedChange={onCheckedChange} aria-label="Autoplay" />);
    expect(toggle).toHaveAttribute("aria-checked", "true");
    expect(toggle).toHaveAttribute("data-state", "checked");

    fireEvent.click(toggle);
    expect(onCheckedChange).toHaveBeenLastCalledWith(false);
  });

  it("fills its track when on and keeps a mid grey when off", () => {
    render(
      <>
        <Switch checked onCheckedChange={vi.fn()} aria-label="On" />
        <Switch checked={false} onCheckedChange={vi.fn()} aria-label="Off" />
      </>,
    );

    const on = screen.getByRole("switch", { name: "On" });
    expect(on).toHaveClass("border-primary", "bg-primary");
    expect(on.firstElementChild).toHaveClass("translate-x-5");

    const off = screen.getByRole("switch", { name: "Off" });
    expect(off).toHaveClass("border-border", "bg-muted-foreground/25");
    expectNoClass(off, "bg-primary", "border-primary");
    expect(off.firstElementChild).toHaveClass("translate-x-1");
  });

  it("does nothing when disabled", () => {
    const onCheckedChange = vi.fn();
    render(
      <Switch checked={false} onCheckedChange={onCheckedChange} disabled aria-label="Autoplay" />,
    );

    const toggle = screen.getByRole("switch", { name: "Autoplay" });
    expect(toggle).toBeDisabled();
    fireEvent.click(toggle);
    expect(onCheckedChange).not.toHaveBeenCalled();
  });

  it("takes layout classes and a ref's worth of button props", () => {
    render(
      <Switch
        checked
        onCheckedChange={vi.fn()}
        aria-labelledby="label"
        aria-describedby="help"
        className="mt-1"
      />,
    );

    const toggle = screen.getByRole("switch");
    expect(toggle).toHaveClass("mt-1", "h-6", "w-11");
    expect(toggle).toHaveAttribute("aria-labelledby", "label");
    expect(toggle).toHaveAttribute("aria-describedby", "help");
  });
});

describe("Skeleton", () => {
  it("is a muted box with a small radius, no role and no pulse", () => {
    render(<Skeleton data-testid="box" className="h-4 w-24" />);

    const box = screen.getByTestId("box");
    expect(box).toHaveClass("rounded-sm", "bg-muted", "h-4", "w-24");
    expect(box).not.toHaveAttribute("role");
    expectNoClass(box, "animate-pulse");
  });

  it("pulses on request, and stops under reduced motion", () => {
    render(<Skeleton pulse data-testid="box" />);

    expect(screen.getByTestId("box")).toHaveClass(
      "animate-pulse",
      "motion-reduce:animate-none",
    );
  });

  it.each(["rounded-full", "rounded-lg", "rounded-none"])(
    "takes %s in place of its own radius",
    (radius) => {
      render(<Skeleton data-testid="box" className={radius} />);

      const box = screen.getByTestId("box");
      expect(box).toHaveClass(radius);
      expect(box).not.toHaveClass("rounded-sm");
    },
  );
});

describe("RadioCard", () => {
  function renderCards(onChange = vi.fn()) {
    render(
      <ul>
        <RadioCard as="li" id="a" name="match" checked onChange={onChange}>
          Signal Fire (2024)
        </RadioCard>
        <RadioCard
          as="li"
          id="b"
          name="match"
          checked={false}
          onChange={onChange}
          aria-describedby="b-meta"
          footer={<p id="b-meta">Already in your library</p>}
        >
          Signal Fire (1998)
        </RadioCard>
      </ul>,
    );
    return onChange;
  }

  it("is a native radio named by the card's content", () => {
    renderCards();

    const picked = screen.getByRole("radio", { name: "Signal Fire (2024)" });
    const other = screen.getByRole("radio", { name: "Signal Fire (1998)" });
    expect(picked).toBeChecked();
    expect(other).not.toBeChecked();
    expect(picked).toHaveAttribute("name", "match");
    expect(picked).toHaveAttribute("aria-labelledby", "a-label");
    // Kept in the tree, out of sight, so the browser's arrow keys work.
    expect(picked).toHaveClass("peer", "sr-only");
  });

  it("picks the card when its label is clicked", () => {
    const onChange = renderCards();

    fireEvent.click(screen.getByText("Signal Fire (1998)"));

    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it("tints the picked card and leaves the other muted", () => {
    renderCards();

    const [picked, other] = screen.getAllByRole("listitem");
    expect(picked).toHaveAttribute("data-state", "checked");
    expect(picked).toHaveClass("border-primary", "bg-primary/10");
    expect(other).toHaveAttribute("data-state", "unchecked");
    expect(other).toHaveClass("border-border", "bg-muted/60");
    expectNoClass(other, "border-primary", "bg-primary/10");
  });

  it("keeps the footer out of the radio's name and ties it in as a description", () => {
    renderCards();

    const other = screen.getByRole("radio", { name: "Signal Fire (1998)" });
    expect(other).toHaveAccessibleDescription("Already in your library");
    const footer = screen.getByText("Already in your library");
    expect(footer.closest("label")).toBeNull();
    expect(footer.closest("li")).toBe(other.closest("li"));
  });

  it("is a div unless asked to be a list item", () => {
    render(
      <RadioCard id="solo" name="solo" checked={false} onChange={vi.fn()}>
        Solo
      </RadioCard>,
    );

    expect(screen.getByRole("radio", { name: "Solo" }).parentElement?.tagName).toBe("DIV");
  });
});
