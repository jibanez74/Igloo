import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import ComingSoon from "@/components/shared/ComingSoon";
import {
  MOTION_DECORATIVE_BOUNCE_CLASS,
  MOTION_DECORATIVE_PING_CLASS,
  MOTION_PAGE_ENTER_CLASS,
} from "@/lib/constants";

describe("ComingSoon", () => {
  it("renders accessible placeholder content", () => {
    const description =
      "Your personal photo gallery is coming soon. Organize, browse, and share your memories all in one place.";

    render(<ComingSoon title="Photos" description={description} />);

    expect(
      screen.getByRole("heading", { name: "Photos" }),
    ).toBeInTheDocument();
    expect(screen.getByText(description)).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(
      "Under Development",
    );

    const announcement = screen.getByText("Photos - Under Development");
    expect(announcement).toHaveAttribute("tabindex", "0");
    expect(announcement).toHaveAttribute(
      "aria-label",
      `Photos. Under Development. ${description}`,
    );
  });

  it("uses the shared entrance class without hidden initial state", () => {
    render(<ComingSoon title="TV Shows" />);

    const content = screen.getByRole("heading", { name: "TV Shows" })
      .parentElement;

    expect(content).not.toBeNull();
    expect(content).toHaveClass(...MOTION_PAGE_ENTER_CLASS.split(" "));
    expect(content).not.toHaveClass("opacity-0");
    expect(content).not.toHaveClass("translate-y-4");
  });

  it("uses reduced-motion-safe decorative animation classes", () => {
    render(<ComingSoon title="Photos" />);

    const decorativeElements = Array.from(
      document.querySelectorAll('[data-motion="decorative"]'),
    );

    expect(decorativeElements).toHaveLength(4);
    expect(decorativeElements[0]).toHaveClass(
      ...MOTION_DECORATIVE_PING_CLASS.split(" "),
    );

    for (const element of decorativeElements.slice(1)) {
      expect(element).toHaveClass(
        ...MOTION_DECORATIVE_BOUNCE_CLASS.split(" "),
      );
    }
  });
});
