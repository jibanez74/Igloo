import type React from "react";
import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import AppShell from "@/components/AppShell";

vi.mock("@/components/app-sidebar", () => ({
  default: () => (
    <nav aria-label="Main navigation">
      <a href="/">Home</a>
    </nav>
  ),
}));

vi.mock("@/components/Header", () => ({
  default: () => <div role="search" aria-label="Search library" />,
}));

vi.mock("@/components/ui/sidebar", () => ({
  SidebarInset: ({
    children,
    ...props
  }: React.ComponentProps<"main">) => <main {...props}>{children}</main>,
  SidebarProvider: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SidebarTrigger: (props: React.ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button type="button" aria-label="Toggle Sidebar" {...props} />
  ),
}));

describe("AppShell", () => {
  it("renders one protected app shell around route content", () => {
    render(
      <AppShell>
        <h1>Route content</h1>
      </AppShell>,
    );

    expect(
      screen.getByRole("navigation", { name: /main navigation/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("search", { name: /search library/i }),
    ).toBeInTheDocument();

    const mainLandmarks = screen.getAllByRole("main");
    expect(mainLandmarks).toHaveLength(1);

    const [main] = mainLandmarks;
    expect(main).toHaveAttribute("id", "main");
    expect(
      within(main).getByRole("heading", { name: /route content/i }),
    ).toBeInTheDocument();
  });
});
