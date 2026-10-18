import { fireEvent, render, screen, within } from "@testing-library/react";
import { Film } from "lucide-react";
import { describe, expect, it, vi } from "vitest";
import HomeMediaSection from "@/components/home/HomeMediaSection";

function renderSection(
  props: Partial<React.ComponentProps<typeof HomeMediaSection<string>>> = {},
) {
  return render(
    <HomeMediaSection<string>
      title="Recently Added Movies"
      headingId="latest-movies"
      items={[]}
      errorMessage={undefined}
      emptyTitle="No movies yet"
      emptyDescription="Scan a library to see movies here."
      emptyIcon={Film}
      countNoun="movie"
      gridClassName="grid"
      getKey={item => item}
      renderItem={item => <article>{item}</article>}
      {...props}
    />,
  );
}

describe("HomeMediaSection", () => {
  it("lists the cards under the section heading's name", () => {
    renderSection({ items: ["Arrival", "Dune"] });

    const list = screen.getByRole("list", { name: "Recently Added Movies" });
    expect(within(list).getAllByRole("listitem")).toHaveLength(2);
  });

  it.each([
    ["pending", { isPending: true }],
    ["failed", { errorMessage: "Couldn’t load movies." }],
    ["empty", {}],
  ] as const)("renders no list while %s", (_state, props) => {
    renderSection(props);
    expect(screen.queryByRole("list")).not.toBeInTheDocument();
  });

  it("renders a failed load as an alert with Try again wired to the refetch", () => {
    const onRetry = vi.fn();
    renderSection({ errorMessage: "Couldn’t load movies.", onRetry });

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("Couldn’t load movies.");
    expect(screen.queryByText(/\d+ movies?/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it("shows no Try again when the section cannot refetch", () => {
    renderSection({ errorMessage: "Couldn’t load movies." });

    expect(screen.getByRole("alert")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Try again" }),
    ).not.toBeInTheDocument();
  });
});
