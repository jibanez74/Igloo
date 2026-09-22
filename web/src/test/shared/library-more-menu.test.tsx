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
    expect(item).not.toHaveAttribute("title");

    await user.click(item);
    expect(onSelect).toHaveBeenCalledOnce();
  });

  // A disabled menu item announces its label and nothing else, so the reason
  // has to travel in the name - and the two reasons are not interchangeable.
  it("says the status is still loading while it is", async () => {
    renderMenu({ available: false, statusLoading: true });

    const item = await screen.findByRole("menuitem", {
      name: "Request Album unavailable. Spotify search status is still loading.",
    });
    expect(item).toHaveAttribute(
      "title",
      "Spotify search status is still loading.",
    );
  });

  it("says the search is unavailable once the status has answered", async () => {
    renderMenu({ available: false, statusLoading: false });

    const item = await screen.findByRole("menuitem", {
      name: "Request Album unavailable. Spotify search is unavailable on this server.",
    });
    expect(item).toHaveAttribute(
      "title",
      "Spotify search is unavailable on this server.",
    );
  });

  it("does not fire while it is disabled", async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();

    renderMenu({ available: false, statusLoading: false, onSelect });

    await user.click(
      await screen.findByRole("menuitem", { name: /Request Album unavailable/ }),
    );

    expect(onSelect).not.toHaveBeenCalled();
  });
});
