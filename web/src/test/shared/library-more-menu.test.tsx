import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import LibraryMoreMenu, {
  RequestMediaMenuItem,
} from "@/components/shared/LibraryMoreMenu";
import { renderWithQueryClient } from "../helpers/render";

function renderMenu(props: {
  available: boolean;
  statusLoading: boolean;
  onSelect?: () => void;
}) {
  return renderWithQueryClient(
    <LibraryMoreMenu open onOpenChange={() => {}}>
      <RequestMediaMenuItem
        label="Request Album"
        provider="Spotify"
        available={props.available}
        statusLoading={props.statusLoading}
        onSelect={props.onSelect ?? (() => {})}
      />
    </LibraryMoreMenu>,
  );
}

describe("RequestMediaMenuItem", () => {
  it("names itself plainly once the provider is known to work", async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();

    renderMenu({ available: true, statusLoading: false, onSelect });

    const item = await screen.findByRole("menuitem", {
      name: "Request Album",
    });
    expect(item).not.toHaveAttribute("aria-disabled");

    await user.click(item);
    expect(onSelect).toHaveBeenCalledOnce();
  });

  // A disabled menu item announces its label and nothing else, so the reason
  // has to travel in the name - and the two reasons are not interchangeable.
  it("says the status is still loading while it is", async () => {
    const user = userEvent.setup();
    renderMenu({ available: false, statusLoading: true });

    const item = await screen.findByRole("menuitem", {
      name: "Request Album unavailable. Spotify search status is still loading.",
    });
    expect(item).toHaveAttribute("aria-disabled", "true");

    await user.keyboard("{ArrowDown}");
    expect(item).toHaveFocus();
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "Spotify search status is still loading.",
    );
  });

  // Radix's own `disabled` would take the item out of the arrow-key order and
  // off the pointer, so its tooltip could never open.
  it("takes focus and shows why it is unavailable", async () => {
    const user = userEvent.setup();
    renderMenu({ available: false, statusLoading: false });

    const item = await screen.findByRole("menuitem", {
      name: "Request Album unavailable. Spotify search is unavailable on this server.",
    });
    expect(item).toHaveAttribute("aria-disabled", "true");
    expect(item).not.toHaveAttribute("data-disabled");

    await user.keyboard("{ArrowDown}");
    expect(item).toHaveFocus();
    expect(await screen.findByRole("tooltip")).toHaveTextContent(
      "Spotify search is unavailable on this server.",
    );
  });

  // Swapping the tooltip out once the status answered remounted the item,
  // so a keyboard user resting on it lost focus.
  it("keeps focus on the item when the status answers that search works", async () => {
    const user = userEvent.setup();
    const props = {
      label: "Request Album",
      provider: "Spotify" as const,
      onSelect: () => {},
    };
    const { rerender } = renderWithQueryClient(
      <LibraryMoreMenu open onOpenChange={() => {}}>
        <RequestMediaMenuItem {...props} available={false} statusLoading />
      </LibraryMoreMenu>,
    );

    const item = await screen.findByRole("menuitem", {
      name: /^Request Album unavailable/,
    });
    await user.keyboard("{ArrowDown}");
    expect(item).toHaveFocus();
    expect(await screen.findByRole("tooltip")).toBeInTheDocument();

    rerender(
      <LibraryMoreMenu open onOpenChange={() => {}}>
        <RequestMediaMenuItem {...props} available statusLoading={false} />
      </LibraryMoreMenu>,
    );

    const ready = screen.getByRole("menuitem", { name: "Request Album" });
    expect(ready).toBe(item);
    expect(ready).toHaveFocus();
    expect(ready).not.toHaveAttribute("aria-disabled");
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
  });

  it("does not fire while it is disabled, and the menu stays open", async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();

    renderMenu({ available: false, statusLoading: false, onSelect });

    const item = await screen.findByRole("menuitem", {
      name: /Request Album unavailable/,
    });
    await user.click(item);
    item.focus();
    await user.keyboard("{Enter}");

    expect(onSelect).not.toHaveBeenCalled();
    expect(item).toBeInTheDocument();
  });
});
