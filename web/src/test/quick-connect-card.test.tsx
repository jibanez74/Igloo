import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import QuickConnectApproveCard from "@/components/QuickConnectApproveCard";

const approveQuickConnectMock = vi.fn();
const showSuccessMock = vi.fn();
const showActionFailedMock = vi.fn();

vi.mock("@/lib/api", () => ({
  approveQuickConnect: (...args: unknown[]) => approveQuickConnectMock(...args),
}));

vi.mock("@/lib/toast-helpers", () => ({
  showSuccess: (...args: unknown[]) => showSuccessMock(...args),
  showActionFailed: (...args: unknown[]) => showActionFailedMock(...args),
}));

function renderCard() {
  const queryClient = new QueryClient({
    defaultOptions: {
      mutations: { retry: false },
      queries: { retry: false },
    },
  });

  render(
    <QueryClientProvider client={queryClient}>
      <QuickConnectApproveCard />
    </QueryClientProvider>,
  );
}

describe("QuickConnectApproveCard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("labels the code input accessibly", () => {
    renderCard();

    expect(
      screen.getByRole("textbox", { name: "Quick Connect code" }),
    ).toBeInTheDocument();
  });

  it("uppercases input, submits the code, and clears on success", async () => {
    approveQuickConnectMock.mockResolvedValue({
      error: false,
      message: "Device approved. It will finish signing in shortly.",
    });

    const user = userEvent.setup();
    renderCard();

    const input = screen.getByRole("textbox", { name: "Quick Connect code" });
    await user.type(input, "xk4t7p");
    expect(input).toHaveValue("XK4T7P");

    await user.click(screen.getByRole("button", { name: "Approve device" }));

    await waitFor(() => {
      expect(approveQuickConnectMock).toHaveBeenCalledWith("XK4T7P");
    });
    await waitFor(() => {
      expect(showSuccessMock).toHaveBeenCalled();
    });
    expect(input).toHaveValue("");
  });

  it("rejects codes that are not six characters without calling the API", async () => {
    const user = userEvent.setup();
    renderCard();

    const input = screen.getByRole("textbox", { name: "Quick Connect code" });
    await user.type(input, "AB");
    await user.click(screen.getByRole("button", { name: "Approve device" }));

    expect(approveQuickConnectMock).not.toHaveBeenCalled();

    const error = await screen.findByRole("alert");
    expect(error).toHaveTextContent("Enter the 6-character code");
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input).toHaveAttribute("aria-describedby", error.id);
  });

  it("shows an actionable message when the code is invalid or expired", async () => {
    approveQuickConnectMock.mockResolvedValue({
      error: true,
      message: "404 - The resource you requested was not found.",
      status: 404,
    });

    const user = userEvent.setup();
    renderCard();

    await user.type(
      screen.getByRole("textbox", { name: "Quick Connect code" }),
      "ZZZZZZ",
    );
    await user.click(screen.getByRole("button", { name: "Approve device" }));

    const error = await screen.findByRole("alert");
    expect(error).toHaveTextContent(/invalid or has expired/);
    expect(showActionFailedMock).toHaveBeenCalled();
  });
});
