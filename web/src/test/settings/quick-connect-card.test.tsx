import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import QuickConnectApproveCard from "@/components/settings/QuickConnectApproveCard";
import { DEVICES_KEY } from "@/lib/constants";
import type { DeviceType } from "@/types";

const lookupQuickConnectMock = vi.fn();
const approveQuickConnectMock = vi.fn();
const getDevicesMock = vi.fn();
const showSuccessMock = vi.fn();
const showActionFailedMock = vi.fn();

vi.mock("@/lib/api", () => ({
  lookupQuickConnect: (...args: unknown[]) => lookupQuickConnectMock(...args),
  approveQuickConnect: (...args: unknown[]) => approveQuickConnectMock(...args),
  getDevices: (...args: unknown[]) => getDevicesMock(...args),
}));

vi.mock("@/lib/toast-helpers", () => ({
  showSuccess: (...args: unknown[]) => showSuccessMock(...args),
  showActionFailed: (...args: unknown[]) => showActionFailedMock(...args),
}));

const PENDING_DEVICE = {
  device_name: "Living Room TV",
  platform: "android_tv",
  app_version: "1.0.0",
};

function makeDevice(id: number, name: string): DeviceType {
  return {
    id,
    name,
    platform: "android_tv",
    app_version: "1.0.0",
    created_at: "2026-07-13 12:00:00",
    last_used_at: "2026-07-13 12:00:00",
    is_current: false,
  };
}

function devicesResponse(devices: DeviceType[]) {
  return { error: false, data: { devices } };
}

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

// userEvent and RTL's waitFor deadlock under vitest fake timers, so the
// fake-timer tests drive the form with synchronous fireEvent and advance
// timers explicitly.
async function flushAsync(ms = 0) {
  await act(async () => {
    // A refetch fired by the last advanced timer resolves in microtasks
    // that land after it; a trailing advance keeps us inside the same act
    // while they propagate through React Query's notifier.
    await vi.advanceTimersByTimeAsync(ms);
    await vi.advanceTimersByTimeAsync(50);
  });
}

async function submitCodeAndConfirm() {
  fireEvent.change(
    screen.getByRole("textbox", { name: "Quick Connect code" }),
    { target: { value: "XK4T7P" } },
  );
  fireEvent.click(screen.getByRole("button", { name: "Continue" }));
  await flushAsync();

  fireEvent.click(screen.getByRole("button", { name: "Approve device" }));
  await flushAsync();
}

describe("QuickConnectApproveCard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    lookupQuickConnectMock.mockResolvedValue({
      error: false,
      data: PENDING_DEVICE,
    });
    approveQuickConnectMock.mockResolvedValue({
      error: false,
      message: "Device approved. It will finish signing in shortly.",
    });
    getDevicesMock.mockResolvedValue(devicesResponse([]));
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

  it("uppercases input and shows the pending device before approving", async () => {
    const user = userEvent.setup();
    renderCard();

    const input = screen.getByRole("textbox", { name: "Quick Connect code" });
    await user.type(input, "xk4t7p");
    expect(input).toHaveValue("XK4T7P");

    await user.click(screen.getByRole("button", { name: "Continue" }));

    expect(lookupQuickConnectMock).toHaveBeenCalledWith("XK4T7P");
    // Nothing is approved until the user confirms the device details.
    expect(approveQuickConnectMock).not.toHaveBeenCalled();

    const heading = await screen.findByRole("heading", {
      name: "Approve this device?",
    });
    expect(heading).toHaveFocus();
    expect(screen.getByText("Living Room TV")).toBeInTheDocument();
    expect(screen.getByText("android_tv")).toBeInTheDocument();
    expect(screen.getByText("1.0.0")).toBeInTheDocument();
  });

  it("returns to the code input with the code preserved when going back", async () => {
    const user = userEvent.setup();
    renderCard();

    await user.type(
      screen.getByRole("textbox", { name: "Quick Connect code" }),
      "XK4T7P",
    );
    await user.click(screen.getByRole("button", { name: "Continue" }));
    await screen.findByRole("heading", { name: "Approve this device?" });

    await user.click(screen.getByRole("button", { name: "Back" }));

    const input = screen.getByRole("textbox", { name: "Quick Connect code" });
    expect(input).toHaveValue("XK4T7P");
    expect(input).toHaveFocus();
    expect(approveQuickConnectMock).not.toHaveBeenCalled();
  });

  it("approves after confirmation and reports the device once it connects", async () => {
    vi.useFakeTimers();
    const queryClient = renderCard();

    // A device the user already had, so the new row is detected by id.
    const existing = makeDevice(1, "Old Phone");
    queryClient.setQueryData([DEVICES_KEY], devicesResponse([existing]));
    getDevicesMock.mockResolvedValue(devicesResponse([existing]));

    await submitCodeAndConfirm();

    expect(approveQuickConnectMock).toHaveBeenCalledWith("XK4T7P");
    expect(screen.getByRole("status")).toHaveTextContent(
      /Waiting for Living Room TV to finish signing in/,
    );

    // First poll still only sees the old device.
    await flushAsync(2000);
    expect(screen.getByRole("status")).toHaveTextContent(/Waiting for/);

    // The device redeems its code and shows up on the next poll.
    getDevicesMock.mockResolvedValue(
      devicesResponse([existing, makeDevice(2, "Living Room TV")]),
    );
    await flushAsync(2000);

    expect(screen.getByRole("status")).toHaveTextContent(
      "Living Room TV is connected",
    );
    expect(showSuccessMock).toHaveBeenCalledWith(
      "Device connected",
      "Living Room TV is now signed in.",
    );

    // Pairing another device resets the card to the code input.
    fireEvent.click(screen.getByRole("button", { name: "Pair another device" }));
    const input = screen.getByRole("textbox", { name: "Quick Connect code" });
    expect(input).toHaveValue("");
    expect(input).toHaveFocus();
  });

  it("does not report prior uncached devices as newly connected", async () => {
    vi.useFakeTimers();
    renderCard();

    const existing = makeDevice(1, "Old Phone");
    getDevicesMock.mockResolvedValue(devicesResponse([existing]));

    await submitCodeAndConfirm();

    expect(approveQuickConnectMock).toHaveBeenCalledWith("XK4T7P");
    expect(screen.getByRole("status")).toHaveTextContent(
      /Waiting for Living Room TV to finish signing in/,
    );
    expect(showSuccessMock).not.toHaveBeenCalled();

    await flushAsync(2000);
    expect(screen.getByRole("status")).toHaveTextContent(/Waiting for/);
    expect(showSuccessMock).not.toHaveBeenCalled();

    getDevicesMock.mockResolvedValue(
      devicesResponse([existing, makeDevice(2, "Living Room TV")]),
    );
    await flushAsync(2000);

    expect(screen.getByRole("status")).toHaveTextContent(
      "Living Room TV is connected",
    );
    expect(showSuccessMock).toHaveBeenCalledWith(
      "Device connected",
      "Living Room TV is now signed in.",
    );
  });

  it("softens the message when the device has not connected before the deadline", async () => {
    vi.useFakeTimers();
    const queryClient = renderCard();

    const existing = makeDevice(1, "Old Phone");
    queryClient.setQueryData([DEVICES_KEY], devicesResponse([existing]));
    getDevicesMock.mockResolvedValue(devicesResponse([existing]));

    await submitCodeAndConfirm();
    expect(screen.getByRole("status")).toHaveTextContent(/Waiting for/);

    // The device never polls; past the deadline the card stops waiting but
    // never reports an error, because the approval itself succeeded.
    await flushAsync(32_000);

    expect(screen.getByRole("status")).toHaveTextContent("Device approved");
    expect(screen.getByRole("status")).toHaveTextContent(
      /should appear under Devices shortly/,
    );
    expect(showSuccessMock).not.toHaveBeenCalled();
  });

  it("rejects codes that are not six characters without calling the API", async () => {
    const user = userEvent.setup();
    renderCard();

    const input = screen.getByRole("textbox", { name: "Quick Connect code" });
    await user.type(input, "AB");
    await user.click(screen.getByRole("button", { name: "Continue" }));

    expect(lookupQuickConnectMock).not.toHaveBeenCalled();

    const error = await screen.findByRole("alert");
    expect(error).toHaveTextContent("Enter the 6-character code");
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input).toHaveAttribute("aria-describedby", error.id);
  });

  it("accepts a pasted code that carries whitespace or dashes", async () => {
    const user = userEvent.setup();
    renderCard();

    const input = screen.getByRole("textbox", { name: "Quick Connect code" });
    await user.click(input);
    await user.paste(" xk4-t7p ");
    expect(input).toHaveValue("XK4T7P");

    await user.click(screen.getByRole("button", { name: "Continue" }));

    expect(lookupQuickConnectMock).toHaveBeenCalledWith("XK4T7P");
  });

  it("rejects characters the code alphabet excludes without calling the API", async () => {
    const user = userEvent.setup();
    renderCard();

    const input = screen.getByRole("textbox", { name: "Quick Connect code" });
    await user.type(input, "XK40OP");
    await user.click(screen.getByRole("button", { name: "Continue" }));

    expect(lookupQuickConnectMock).not.toHaveBeenCalled();

    const error = await screen.findByRole("alert");
    expect(error).toHaveTextContent("Codes never contain I, L, O, 0 or 1");
    expect(input).toHaveAttribute("aria-invalid", "true");
    expect(input).toHaveAttribute("aria-describedby", error.id);
  });

  it("shows an actionable message when the looked-up code is invalid or expired", async () => {
    lookupQuickConnectMock.mockResolvedValue({
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
    await user.click(screen.getByRole("button", { name: "Continue" }));

    const error = await screen.findByRole("alert");
    expect(error).toHaveTextContent(/invalid or has expired/);
    expect(
      screen.getByRole("textbox", { name: "Quick Connect code" }),
    ).toBeInTheDocument();
  });

  it("returns to the code input when the code expires between lookup and approval", async () => {
    approveQuickConnectMock.mockResolvedValue({
      error: true,
      message: "404 - The resource you requested was not found.",
      status: 404,
    });

    const user = userEvent.setup();
    renderCard();

    await user.type(
      screen.getByRole("textbox", { name: "Quick Connect code" }),
      "XK4T7P",
    );
    await user.click(screen.getByRole("button", { name: "Continue" }));
    await screen.findByRole("heading", { name: "Approve this device?" });

    await user.click(screen.getByRole("button", { name: "Approve device" }));

    const error = await screen.findByRole("alert");
    expect(error).toHaveTextContent(/invalid or has expired/);
    expect(
      screen.getByRole("textbox", { name: "Quick Connect code" }),
    ).toBeInTheDocument();
    expect(showActionFailedMock).toHaveBeenCalled();
  });
});
