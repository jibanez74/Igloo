import { render, screen } from "@testing-library/react";
import { AlertTriangle, Users } from "lucide-react";
import { describe, expect, it } from "vitest";
import SettingsCardHeader from "@/components/settings/SettingsCardHeader";

describe("SettingsCardHeader", () => {
  it("titles the card with an h2 and a quiet description", () => {
    render(
      <SettingsCardHeader
        icon={Users}
        title="User management"
        description="Create, edit, and remove user accounts."
      />,
    );

    const title = screen.getByRole("heading", {
      level: 2,
      name: "User management",
    });
    expect(title.querySelector("svg")).toHaveClass("text-primary");
    const description = screen.getByText(
      "Create, edit, and remove user accounts.",
    );
    expect(description).toHaveClass("text-muted-foreground");
    expect(description).not.toHaveClass("text-destructive");
  });

  it("holds the card's own action at the end of its row", () => {
    render(
      <SettingsCardHeader
        icon={Users}
        title="User management"
        action={<button type="button">Add user</button>}
      />,
    );

    const header = screen
      .getByRole("heading", { name: "User management" })
      .closest('[data-slot="card-header"]');
    expect(header).toContainElement(
      screen.getByRole("button", { name: "Add user" }),
    );
    // Stacked on a phone, one row from `sm`.
    expect(header).toHaveClass("flex-col", "sm:flex-row", "sm:justify-between");
  });

  it("draws the Danger zone in the destructive color throughout", () => {
    render(
      <SettingsCardHeader
        icon={AlertTriangle}
        title="Danger zone"
        description="Actions here cannot be undone."
        tone="destructive"
      />,
    );

    const title = screen.getByRole("heading", { level: 2, name: "Danger zone" });
    expect(title).toHaveClass("text-destructive");
    // The icon takes the title's color, not the primary one.
    expect(title.querySelector("svg")).not.toHaveClass("text-primary");
    expect(screen.getByText("Actions here cannot be undone.")).toHaveClass(
      "text-destructive",
    );
  });
});
