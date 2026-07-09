import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import QuickConnectApproveCard from "@/components/QuickConnectApproveCard";
import { DEVICES_KEY } from "@/lib/constants";

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

  return queryClient;
}

describe("QuickConnectApproveCard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.useRealTimers();
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

  it("invalidates the devices list immediately and again after the device polls", async () => {
    approveQuickConnectMock.mockResolvedValue({
      error: false,
      message: "Device approved. It will finish signing in shortly.",
    });

    // userEvent and RTL's waitFor deadlock under vitest fake timers, so drive
    // the form with synchronous fireEvent and advance timers explicitly.
    vi.useFakeTimers();
    const queryClient = renderCard();
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");

    fireEvent.change(
      screen.getByRole("textbox", { name: "Quick Connect code" }),
      { target: { value: "XK4T7P" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Approve device" }));

    // The list refreshes right away so an already-redeemed device shows up...
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    expect(invalidateSpy).toHaveBeenCalledTimes(1);
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: [DEVICES_KEY] });

    // ...and again after the device has had time to poll and redeem its code.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(5000);
    });
    expect(invalidateSpy).toHaveBeenCalledTimes(2);
    expect(invalidateSpy).toHaveBeenLastCalledWith({ queryKey: [DEVICES_KEY] });
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
