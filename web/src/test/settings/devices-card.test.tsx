import { QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import DevicesCard from "@/components/settings/DevicesCard";
import type { DeviceType } from "@/types";
import { createTestQueryClient } from "../helpers/render";

const getDevicesMock = vi.fn();
const renameDeviceMock = vi.fn();
const revokeDeviceMock = vi.fn();
const showSuccessMock = vi.fn();
const showActionFailedMock = vi.fn();

vi.mock("@/lib/api", () => ({
  getDevices: (...args: unknown[]) => getDevicesMock(...args),
  renameDevice: (...args: unknown[]) => renameDeviceMock(...args),
  revokeDevice: (...args: unknown[]) => revokeDeviceMock(...args),
}));

vi.mock("@/lib/toast-helpers", () => ({
  showSuccess: (...args: unknown[]) => showSuccessMock(...args),
  showActionFailed: (...args: unknown[]) => showActionFailedMock(...args),
}));

function device(overrides: Partial<DeviceType> = {}): DeviceType {
  return {
    id: 1,
    name: "Living Room TV",
    platform: "android_tv",
    app_version: "1.0.0",
    created_at: "2026-07-01 10:00:00",
    last_used_at: "2026-07-05 20:00:00",
    is_current: false,
    ...overrides,
  };
}

function renderCard() {
  const queryClient = createTestQueryClient();

  render(
    <QueryClientProvider client={queryClient}>
      <DevicesCard />
    </QueryClientProvider>,
  );
}

describe("DevicesCard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows an empty state when no devices are paired", async () => {
    getDevicesMock.mockResolvedValue({ error: false, data: { devices: [] } });

    renderCard();

    expect(
      await screen.findByText(/No devices connected/),
    ).toBeInTheDocument();
  });

  it("lists devices with accessible rename and revoke buttons", async () => {
    getDevicesMock.mockResolvedValue({
      error: false,
      data: { devices: [device(), device({ id: 2, name: "Pixel", platform: "android" })] },
    });

    renderCard();

    const list = await screen.findByRole("list");
    expect(list).toBeInTheDocument();
    expect(screen.getAllByRole("listitem")).toHaveLength(2);

    expect(
      screen.getByRole("button", { name: "Revoke Living Room TV" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Rename Pixel" }),
    ).toBeInTheDocument();
  });

  it("shows the platform with the app version when one is reported", async () => {
    getDevicesMock.mockResolvedValue({
      error: false,
      data: {
        devices: [
          device(),
          device({ id: 2, name: "Pixel", platform: "android", app_version: null }),
        ],
      },
    });

    renderCard();

    expect(await screen.findByText("android_tv · 1.0.0")).toBeInTheDocument();
    // A device without a reported version shows just the platform.
    expect(screen.getByText("android")).toBeInTheDocument();
  });

  it("revokes a device after confirmation", async () => {
    getDevicesMock.mockResolvedValue({
      error: false,
      data: { devices: [device()] },
    });
    revokeDeviceMock.mockResolvedValue({ error: false, message: "Device revoked" });

    const user = userEvent.setup();
    renderCard();

    await user.click(
      await screen.findByRole("button", { name: "Revoke Living Room TV" }),
    );

    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveAccessibleName("Revoke Living Room TV?");

    await user.click(screen.getByRole("button", { name: "Revoke" }));

    await waitFor(() => {
      expect(revokeDeviceMock).toHaveBeenCalledWith(1);
    });
    await waitFor(() => {
      expect(showSuccessMock).toHaveBeenCalledWith("Device revoked");
    });
  });

  it("shows a failure toast and closes the dialog when revoke fails", async () => {
    getDevicesMock.mockResolvedValue({
      error: false,
      data: { devices: [device()] },
    });
    revokeDeviceMock.mockResolvedValue({
      error: true,
      message: "something went wrong",
    });

    const user = userEvent.setup();
    renderCard();

    await user.click(
      await screen.findByRole("button", { name: "Revoke Living Room TV" }),
    );
    await screen.findByRole("alertdialog");

    await user.click(screen.getByRole("button", { name: "Revoke" }));

    await waitFor(() => {
      expect(showActionFailedMock).toHaveBeenCalledWith(
        "revoke device",
        "something went wrong",
      );
    });
    expect(showSuccessMock).not.toHaveBeenCalled();

    // onSettled resets the pending device, closing the confirm dialog.
    await waitFor(() => {
      expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
    });
  });

  it("renames a device inline", async () => {
    getDevicesMock.mockResolvedValue({
      error: false,
      data: { devices: [device()] },
    });
    renameDeviceMock.mockResolvedValue({ error: false, message: "Device renamed" });

    const user = userEvent.setup();
    renderCard();

    await user.click(
      await screen.findByRole("button", { name: "Rename Living Room TV" }),
    );

    const input = screen.getByRole("textbox", {
      name: "New name for Living Room TV",
    });
    await user.clear(input);
    await user.type(input, "Bedroom TV");
    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      expect(renameDeviceMock).toHaveBeenCalledWith(1, "Bedroom TV");
    });
  });

  it("surfaces list errors", async () => {
    getDevicesMock.mockResolvedValue({
      error: true,
      message: "500 - A network error occurred while processing your request.",
    });

    renderCard();

    const error = await screen.findByRole("alert");
    expect(error).toHaveTextContent(/network error/);
  });
});
