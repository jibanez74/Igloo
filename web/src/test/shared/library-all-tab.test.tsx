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

function renderTab(queryFn: () => Promise<ApiResponseType<Payload>>) {
  const opts = queryOptions({
    queryKey: ["library-all-tab-test", 1],
    queryFn,
  });

  return renderWithQueryClient(
    <LibraryAllTab
      queryOpts={opts}
      getItems={data => data.items}
      renderCard={item => <article>{item.name}</article>}
      currentPage={1}
      sort="asc"
      perPage={24}
      noun={NOUN}
      emptyIcon={Tv}
      onPageChange={() => {}}
      onSortToggle={() => {}}
    />,
  );
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
    expect(
      screen.queryByRole("button", { name: /Sorted A to Z/ }),
    ).not.toBeInTheDocument();
  });
});
