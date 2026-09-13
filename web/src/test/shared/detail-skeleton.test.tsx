import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import DetailSkeleton from "@/components/shared/DetailSkeleton";
import { DETAIL_HERO_CONTENT_NO_ACTIONS_CLASS } from "@/lib/constants";

describe("DetailSkeleton", () => {
  it("is a single labelled status region", () => {
    render(<DetailSkeleton label="Loading show details" withActions={false} />);

    expect(
      screen.getByRole("status", { name: "Loading show details" }),
    ).toBeInTheDocument();
  });

  it("takes the no-actions hero padding when the real hero has no actions row", () => {
    const { container } = render(
      <DetailSkeleton label="Loading show details" withActions={false} />,
    );

    // Same bottom padding as DetailHero without an actions slot, so content
    // arrival shifts nothing (design-system §3.4).
    for (const cls of DETAIL_HERO_CONTENT_NO_ACTIONS_CLASS.split(" ")) {
      expect(container.querySelector(`.${cls.replace(":", "\\:")}`)).not.toBeNull();
    }
    expect(container.querySelectorAll(".h-11")).toHaveLength(0);
  });

  it("reserves the action-button row when the real hero has one", () => {
    const { container } = render(
      <DetailSkeleton label="Loading movie details" withActions />,
    );

    expect(container.querySelectorAll(".h-11")).toHaveLength(3);
    expect(container.querySelector(".pb-24")).toBeNull();
  });
});
