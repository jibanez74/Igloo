import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import LibraryPagination from "@/components/LibraryPagination";

describe("LibraryPagination", () => {
  it("keeps pagination controls in a predictable tab order", async () => {
    const user = userEvent.setup();

    render(
      <LibraryPagination
        currentPage={2}
        totalPages={5}
        onPageChange={vi.fn()}
      />,
    );

    const previousButton = screen.getByRole("button", {
      name: /go to previous page/i,
    });
    const pageOneButton = screen.getByRole("button", {
      name: /go to page 1/i,
    });
    const currentPageButton = screen.getByRole("button", {
      name: /page 2, current page/i,
    });

    await user.tab();
    expect(previousButton).toHaveFocus();

    await user.tab();
    expect(pageOneButton).toHaveFocus();

    await user.tab();
    expect(currentPageButton).toHaveFocus();
    expect(currentPageButton).toHaveAttribute("aria-current", "page");
  });

  it("activates the focused pagination control with Enter", async () => {
    const onPageChange = vi.fn();
    const user = userEvent.setup();

    render(
      <LibraryPagination
        currentPage={2}
        totalPages={5}
        onPageChange={onPageChange}
      />,
    );

    const previousButton = screen.getByRole("button", {
      name: /go to previous page/i,
    });
    const pageOneButton = screen.getByRole("button", {
      name: /go to page 1/i,
    });
    const currentPageButton = screen.getByRole("button", {
      name: /page 2, current page/i,
    });
    const pageThreeButton = screen.getByRole("button", {
      name: /go to page 3/i,
    });

    currentPageButton.focus();
    expect(currentPageButton).toHaveAttribute("aria-current", "page");

    await user.keyboard("{Enter}");
    expect(onPageChange).not.toHaveBeenCalled();

    pageThreeButton.focus();
    expect(pageThreeButton).toHaveFocus();
    await user.keyboard("{Enter}");
    expect(onPageChange).toHaveBeenCalledWith(3);
  });

  it("disables previous and next controls at the page boundaries", async () => {
    const onPageChange = vi.fn();
    const user = userEvent.setup();
    const { rerender } = render(
      <LibraryPagination
        currentPage={1}
        totalPages={3}
        onPageChange={onPageChange}
      />,
    );

    const previousButton = screen.getByRole("button", {
      name: /go to previous page/i,
    });

    expect(previousButton).toBeDisabled();

    await user.click(previousButton);
    expect(onPageChange).not.toHaveBeenCalled();

    rerender(
      <LibraryPagination
        currentPage={3}
        totalPages={3}
        onPageChange={onPageChange}
      />,
    );

    expect(
      screen.getByRole("button", { name: /go to next page/i }),
    ).toBeDisabled();
  });
});
