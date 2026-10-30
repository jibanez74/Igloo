import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Download } from "lucide-react";
import { describe, expect, it, vi } from "vitest";
import SwitchField from "@/components/settings/SwitchField";

function renderSwitch(checked: boolean, onCheckedChange = vi.fn()) {
  render(
    <SwitchField
      id="download-images"
      label="Download images"
      description="Save library artwork in the static directory."
      checked={checked}
      onCheckedChange={onCheckedChange}
      icon={<Download aria-hidden="true" />}
    />,
  );
  return {
    control: screen.getByRole("switch", { name: "Download images" }),
    onCheckedChange,
  };
}

describe("SwitchField", () => {
  it("is a described switch that reports the next state on click", async () => {
    const { control, onCheckedChange } = renderSwitch(false);

    expect(control).toHaveAttribute("aria-checked", "false");
    expect(control).toHaveAccessibleDescription(
      "Save library artwork in the static directory.",
    );
    await userEvent.click(control);
    expect(onCheckedChange).toHaveBeenCalledWith(true);
  });

  it("keeps the off track a mid grey in both themes so the white thumb stays visible (design-system §3.7)", () => {
    const { control } = renderSwitch(false);

    expect(control).toHaveClass("bg-muted-foreground/25");
    expect(control).not.toHaveClass("bg-accent");
    expect(control.querySelector(".bg-white")).not.toBeNull();
  });

  it("fills the track with the primary colour when on", () => {
    const { control } = renderSwitch(true);

    expect(control).toHaveAttribute("aria-checked", "true");
    expect(control).toHaveClass("bg-primary");
    expect(control).not.toHaveClass("bg-muted-foreground/25");
  });
});
