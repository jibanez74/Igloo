import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { FIELD_VARIANTS } from "@/components/ui/field-variants";
import { Input } from "@/components/ui/input";
import { NativeSelect } from "@/components/ui/native-select";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { lightInputClassName } from "@/lib/input-styles";

// One set of looks for every kind of field (design-system §1.6, §3.7), so a
// form that mixes an input, a textarea and a select reads as one, and a call
// site names the look instead of restating its classes.
const classesOf = (value: string) => value.split(" ").filter(Boolean);

describe("field variants", () => {
  it("fills a field by default", () => {
    render(
      <>
        <Input aria-label="Name" />
        <Textarea aria-label="Notes" />
        <NativeSelect aria-label="Sort" />
      </>,
    );

    for (const name of ["Name", "Notes"]) {
      const field = screen.getByRole("textbox", { name });
      expect(field).toHaveAttribute("data-variant", "filled");
      expect(field).toHaveClass("border-border", "bg-muted", "text-foreground");
      // The variant's border replaces the base one; both would leave the
      // winner to stylesheet order.
      expect(field).not.toHaveClass("border-input");
    }
    expect(screen.getByRole("combobox", { name: "Sort" })).toHaveClass(
      "border-border",
      "bg-muted",
      "text-foreground",
    );
  });

  it.each(["inset", "glass", "outline"] as const)(
    "gives an input, a textarea and a native select the %s look",
    variant => {
      render(
        <>
          <Input aria-label="Name" variant={variant} />
          <Textarea aria-label="Notes" variant={variant} />
          <NativeSelect aria-label="Sort" variant={variant} />
        </>,
      );

      const fields = [
        screen.getByRole("textbox", { name: "Name" }),
        screen.getByRole("textbox", { name: "Notes" }),
        screen.getByRole("combobox", { name: "Sort" }),
      ];
      for (const field of fields) {
        expect(field).toHaveAttribute("data-variant", variant);
        // Outline adds nothing: it is the base field.
        const classes = classesOf(FIELD_VARIANTS[variant]);
        if (classes.length > 0) expect(field).toHaveClass(...classes);
        expect(field).not.toHaveClass("bg-muted");
      }
    },
  );

  it("keeps the bare field's own border and no fill for outline", () => {
    render(<Input aria-label="Code" variant="outline" />);

    const field = screen.getByRole("textbox", { name: "Code" });
    expect(field).toHaveClass("border-input", "bg-transparent");
    expect(field).not.toHaveClass("bg-muted", "border-border");
  });

  it("draws the glass look from the shared light-input recipe", () => {
    expect(FIELD_VARIANTS.glass).toBe(lightInputClassName);
  });
});

describe("Input size", () => {
  it.each([
    ["sm", "h-8"],
    ["default", "h-9"],
    ["lg", "h-10"],
  ] as const)("is %s at %s", (size, height) => {
    render(<Input aria-label="Name" size={size} />);

    const field = screen.getByRole("textbox", { name: "Name" });
    expect(field).toHaveClass(height);
    for (const other of ["h-8", "h-9", "h-10"]) {
      if (other !== height) expect(field).not.toHaveClass(other);
    }
  });

  it("does not pass its size to the native attribute", () => {
    render(<Input aria-label="Name" size="lg" />);

    expect(screen.getByRole("textbox", { name: "Name" })).not.toHaveAttribute(
      "size",
    );
  });

  it("lets className add layout without restating the look", () => {
    render(<Input aria-label="Search" variant="glass" className="pl-10" />);

    const field = screen.getByRole("textbox", { name: "Search" });
    expect(field).toHaveClass("pl-10", ...classesOf(lightInputClassName));
  });
});

describe("Textarea and NativeSelect", () => {
  it("renders the native elements, so forms and screen readers see them as such", () => {
    render(
      <>
        <Textarea aria-label="Notes" rows={3} disabled />
        <NativeSelect aria-label="Sort" defaultValue="b">
          <option value="a">A to Z</option>
          <option value="b">Z to A</option>
        </NativeSelect>
      </>,
    );

    const textarea = screen.getByRole("textbox", { name: "Notes" });
    expect(textarea.tagName).toBe("TEXTAREA");
    expect(textarea).toHaveAttribute("rows", "3");
    expect(textarea).toBeDisabled();
    expect(textarea).toHaveClass("disabled:opacity-50");

    const select = screen.getByRole("combobox", { name: "Sort" });
    expect(select.tagName).toBe("SELECT");
    expect(select).toHaveValue("b");
    expect(screen.getAllByRole("option")).toHaveLength(2);
  });

  it("carries the shared focus ring", () => {
    render(
      <>
        <Textarea aria-label="Notes" />
        <NativeSelect aria-label="Sort" />
      </>,
    );

    for (const field of [
      screen.getByRole("textbox", { name: "Notes" }),
      screen.getByRole("combobox", { name: "Sort" }),
    ]) {
      expect(field).toHaveClass(
        "outline-hidden",
        "focus-visible:border-ring",
        "focus-visible:ring-[3px]",
        "focus-visible:ring-ring/50",
      );
    }
  });
});

describe("SelectTrigger variants", () => {
  const renderTrigger = (
    props: Pick<React.ComponentProps<typeof SelectTrigger>, "variant" | "size">,
  ) => {
    render(
      <Select value="a" onValueChange={() => {}}>
        <SelectTrigger aria-label="Profile" {...props}>
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="a">Use recommended</SelectItem>
        </SelectContent>
      </Select>,
    );
    return screen.getByRole("combobox", { name: "Profile" });
  };

  it("fills the trigger and spans its container by default", () => {
    const trigger = renderTrigger({});

    expect(trigger).toHaveAttribute("data-variant", "filled");
    expect(trigger).toHaveClass(
      "w-full",
      "min-w-0",
      "h-9",
      "border-border",
      "bg-muted",
      "text-foreground",
    );
  });

  it("sits flat on its card when inset, at the large height Settings use", () => {
    const trigger = renderTrigger({ variant: "inset", size: "lg" });

    expect(trigger).toHaveClass(
      "h-10",
      "border-border",
      "bg-background/60",
      "shadow-none",
    );
    expect(trigger).not.toHaveClass("h-9", "shadow-xs", "bg-muted");
  });

  it("keeps the bare trigger for outline", () => {
    const trigger = renderTrigger({ variant: "outline" });

    expect(trigger).toHaveClass("border-input", "bg-transparent", "shadow-xs");
    expect(trigger).not.toHaveClass("bg-muted");
  });
});
