import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import PlaylistFormDialog from "@/components/PlaylistFormDialog";

function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      mutations: { retry: false },
      queries: { retry: false },
    },
  });
}

describe("PlaylistFormDialog", () => {
  it("gives the description textarea an accessible name from the visible label", () => {
    const queryClient = createQueryClient();

    render(
      <QueryClientProvider client={queryClient}>
        <PlaylistFormDialog mode="create" open onOpenChange={vi.fn()} />
      </QueryClientProvider>,
    );

    expect(
      screen.getByRole("textbox", { name: /^Description/ }),
    ).toHaveAccessibleName("Description (optional)");
  });
});
