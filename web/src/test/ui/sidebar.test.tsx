import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
  Sidebar,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar";

// useIsMobile reads window.innerWidth on its first render, so the width set
// before render decides which sidebar the provider builds.
const desktopWidth = window.innerWidth;

function renderSidebar() {
  render(
    <SidebarProvider>
      <Sidebar>
        <a href="/movies">Movies</a>
      </Sidebar>
      <SidebarTrigger />
    </SidebarProvider>,
  );
}

describe("Sidebar on a phone", () => {
  beforeEach(() => {
    window.innerWidth = 500;
  });

  afterEach(() => {
    window.innerWidth = desktopWidth;
  });

  it("opens the navigation as a modal sheet and closes it with Escape", async () => {
    const user = userEvent.setup();
    renderSidebar();

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    const trigger = screen.getByRole("button", { name: "Open navigation" });
    expect(trigger).toHaveAttribute("aria-expanded", "false");

    await user.click(trigger);

    const sheet = screen.getByRole("dialog", { name: "Navigation" });
    expect(sheet).toHaveAttribute("data-mobile", "true");
    expect(sheet).toContainElement(screen.getByRole("link", { name: "Movies" }));
    expect(trigger).toHaveAttribute("aria-expanded", "true");

    await user.keyboard("{Escape}");

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(trigger).toHaveAttribute("aria-expanded", "false");
  });
});

describe("Sidebar on a wide screen", () => {
  it("stays in the page instead of opening a sheet", () => {
    renderSidebar();

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Movies" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Collapse sidebar" })).toBeInTheDocument();
  });
});
