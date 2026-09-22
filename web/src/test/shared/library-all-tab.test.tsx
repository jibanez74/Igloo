import { queryOptions } from "@tanstack/react-query";
import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Tv } from "lucide-react";
import { describe, expect, it, vi } from "vitest";
import LibraryAllTab from "@/components/shared/LibraryAllTab";
import type { ApiResponseType } from "@/types";
import { renderWithQueryClient } from "../helpers/render";

type Item = { id: number; name: string };
type Payload = {
  items: Item[];
  total: number;
  page: number;
  per_page: number;
  total_pages: number;
};

const NOUN = { singular: "show", plural: "shows" };

function renderTab(
  queryFn: () => Promise<ApiResponseType<Payload>>,
  {
    currentPage = 1,
    onPageChange = () => {},
  }: { currentPage?: number; onPageChange?: (page: number) => void } = {},
) {
  const opts = queryOptions({
    queryKey: ["library-all-tab-test", currentPage],
    queryFn,
  });

  return renderWithQueryClient(
    <LibraryAllTab
      queryOpts={opts}
      getItems={data => data.items}
      renderCard={item => <article>{item.name}</article>}
      currentPage={currentPage}
      sort="asc"
      perPage={24}
      noun={NOUN}
      emptyIcon={Tv}
      onPageChange={onPageChange}
      onSortToggle={() => {}}
    />,
  );
}

function onePage(items: Item[]): ApiResponseType<Payload> {
  return {
    error: false,
    data: {
      items,
      total: items.length,
      page: 1,
      per_page: 24,
      total_pages: items.length > 0 ? 1 : 0,
    },
  };
}

// The route tests cover the happy path; these are the branches a loader-fed
// route never reaches, because the loader would have thrown first.
describe("LibraryAllTab", () => {
  it("renders the API error envelope as an alert and refetches on retry", async () => {
    const user = userEvent.setup();
    const queryFn = vi
      .fn<() => Promise<ApiResponseType<Payload>>>()
      .mockResolvedValueOnce({ error: true, message: "Shows are unavailable." })
      .mockResolvedValueOnce({
        error: false,
        data: {
          items: [{ id: 1, name: "Frost Harbor" }],
          total: 1,
          page: 1,
          per_page: 24,
          total_pages: 1,
        },
      });

    renderTab(queryFn);

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Shows are unavailable.");

    await user.click(screen.getByRole("button", { name: "Try again" }));

    await waitFor(() => {
      expect(screen.getByText("Frost Harbor")).toBeInTheDocument();
    });
    expect(queryFn).toHaveBeenCalledTimes(2);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("renders the minimal empty state when the page has no items", async () => {
    renderTab(async () => ({
      error: false,
      data: { items: [], total: 0, page: 1, per_page: 24, total_pages: 0 },
    }));

    expect(
      await screen.findByText("No shows found in your library."),
    ).toBeInTheDocument();
    // The toolbar is reserved in every state, so it outlives the grid.
    expect(
      screen.getByRole("button", { name: /Sorted A to Z/ }),
    ).toBeInTheDocument();
  });

  it("walks back to the last page when the requested page is out of range", async () => {
    const onPageChange = vi.fn();

    renderTab(
      async () => ({
        error: false,
        data: { items: [], total: 25, page: 5, per_page: 24, total_pages: 2 },
      }),
      { currentPage: 5, onPageChange },
    );

    await waitFor(() => {
      expect(onPageChange).toHaveBeenCalledWith(2);
    });
  });

  it("stays on the empty state when the library itself is empty", async () => {
    const onPageChange = vi.fn();

    renderTab(
      async () => ({
        error: false,
        data: { items: [], total: 0, page: 1, per_page: 24, total_pages: 0 },
      }),
      { onPageChange },
    );

    expect(
      await screen.findByText("No shows found in your library."),
    ).toBeInTheDocument();
    expect(onPageChange).not.toHaveBeenCalled();
  });

  it("announces the empty state to screen readers", async () => {
    renderTab(async () => onePage([]));

    await screen.findByText("No shows found in your library.");

    await waitFor(() => {
      const statusRegions = screen.getAllByRole("status");
      expect(
        statusRegions.some(region => region.textContent === "No shows found"),
      ).toBe(true);
    });
  });

  it("announces a single result with the singular noun", async () => {
    renderTab(async () => onePage([{ id: 1, name: "Frost Harbor" }]));

    await screen.findByText("Frost Harbor");

    await waitFor(() => {
      const statusRegions = screen.getAllByRole("status");
      expect(
        statusRegions.some(
          region => region.textContent === "Showing 1 show, page 1 of 1",
        ),
      ).toBe(true);
    });
  });
});

// A list whose API does not sort (albums, musicians) passes no sort pair and
// swaps the poster grid for its own card geometry.
describe("LibraryAllTab without sort", () => {
  let unsortedKey = 0;

  function renderUnsortedTab(
    queryFn: () => Promise<ApiResponseType<Payload>>,
  ) {
    unsortedKey += 1;
    const opts = queryOptions({
      queryKey: ["library-all-tab-unsorted-test", unsortedKey],
      queryFn,
      retry: false,
    });

    return renderWithQueryClient(
      <LibraryAllTab
        queryOpts={opts}
        getItems={data => data.items}
        renderCard={item => <article>{item.name}</article>}
        currentPage={1}
        perPage={3}
        noun={NOUN}
        emptyIcon={Tv}
        gridClassName="test-grid"
        skeletonCard={<div data-testid="round-card" />}
        onPageChange={() => {}}
      />,
    );
  }

  it("repeats the supplied skeleton card in the supplied grid while loading", () => {
    const { container } = renderUnsortedTab(() => new Promise(() => {}));

    expect(screen.getAllByTestId("round-card")).toHaveLength(3);
    expect(container.querySelector(".test-grid")).not.toBeNull();
    // The skeleton mirrors the grid only; the toolbar above it is the tab's,
    // so there is no placeholder pill standing in for the sort toggle.
    expect(container.querySelector(".h-8.w-16")).toBeNull();
  });

  it("renders the grid without a sort toggle or page header once loaded", async () => {
    const { container } = renderUnsortedTab(async () =>
      onePage([{ id: 1, name: "Frost Harbor" }]),
    );

    await screen.findByText("Frost Harbor");

    expect(
      screen.queryByRole("button", { name: /Sorted/ }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/Page 1 of/)).not.toBeInTheDocument();
    expect(
      container.querySelector(".test-grid")?.contains(
        screen.getByText("Frost Harbor"),
      ),
    ).toBe(true);
  });

  it("reserves the toolbar row before an unsorted paginated tab resolves", async () => {
    let resolve: (value: ApiResponseType<Payload>) => void = () => {};
    const { container } = renderUnsortedTab(
      () =>
        new Promise<ApiResponseType<Payload>>(r => {
          resolve = r;
        }),
    );

    const toolbarSelector = '[data-slot="library-tab-toolbar"]';
    expect(container.querySelector(toolbarSelector)).not.toBeNull();

    resolve({
      error: false,
      data: {
        items: [{ id: 1, name: "Frost Harbor" }],
        total: 72,
        page: 1,
        per_page: 3,
        total_pages: 3,
      },
    });

    await screen.findByText("Frost Harbor");

    // Same row, now carrying the page info it had reserved room for.
    expect(container.querySelector(toolbarSelector)).not.toBeNull();
    expect(screen.getByText("Page 1 of 3")).toBeInTheDocument();
  });

  it("keeps the toolbar row on the error state", async () => {
    const { container } = renderUnsortedTab(async () => {
      throw new Error("offline");
    });

    await screen.findByRole("alert");

    expect(
      container.querySelector('[data-slot="library-tab-toolbar"]'),
    ).not.toBeNull();
  });
});
