import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import SettingsPage from "@/components/settings/SettingsPage";

// Every Settings tab sits in the same column: two tabs used to run the full
// width with a wider gap between their cards.

describe("SettingsPage", () => {
  it("is one width and one gap, whatever the tab", () => {
    render(
      <SettingsPage data-testid="page">
        <p>card</p>
      </SettingsPage>,
    );

    const page = screen.getByTestId("page");
    expect(page.tagName).toBe("DIV");
    expect(page).toHaveClass("@container", "max-w-5xl", "space-y-6");
  });

  it("is the form itself for a tab saved by a Save bar", () => {
    const onSubmit = vi.fn((event: React.FormEvent) => event.preventDefault());
    render(
      <SettingsPage as="form" data-testid="page" noValidate onSubmit={onSubmit}>
        <button type="submit">Save</button>
      </SettingsPage>,
    );

    const page = screen.getByTestId("page");
    expect(page.tagName).toBe("FORM");
    expect(page).toHaveAttribute("novalidate");
    expect(page).toHaveClass("max-w-5xl", "space-y-6");
    expect(page).not.toHaveAttribute("as");

    fireEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  // A sticky Save bar covers the bottom of the viewport, so a control the
  // browser scrolls to on focus must stop above it.
  it("keeps focused controls clear of a sticky Save bar", () => {
    render(<SettingsPage data-testid="page" />);

    expect(screen.getByTestId("page").className).toContain(
      "[&_:is(button,input,select)]:scroll-mb-72",
    );
  });
});
