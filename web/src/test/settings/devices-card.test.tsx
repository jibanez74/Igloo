import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import DevicesCard from "@/components/settings/DevicesCard";
import type { DeviceType } from "@/types";
import { createTestQueryClient, renderWithQueryClient } from "../helpers/render";

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
    ...overrides,
  };
}

function renderCard() {
  const queryClient = createTestQueryClient();

  renderWithQueryClient(
    <DevicesCard />,
    { queryClient },
  );
}

describe("DevicesCard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.unstubAllEnvs();
  });

  it("dates a device by the UTC day the server recorded, not local time", async () => {
    // 23:30 UTC on July 5th is already July 6th in Tokyo; read as local
    // time it would stay on the 5th.
    vi.stubEnv("TZ", "Asia/Tokyo");
    getDevicesMock.mockResolvedValue({
      error: false,
      data: { devices: [device({ last_used_at: "2026-07-05 23:30:00" })] },
    });

    renderCard();

    const lastUsed = new Date(Date.UTC(2026, 6, 5, 23, 30)).toLocaleDateString();
    expect(await screen.findByText(/Last used/)).toHaveTextContent(
      `Last used ${lastUsed}`,
    );
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
        expect.objectContaining({ error: true, message: "something went wrong" }),
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

  // Closing these used to drop focus on the page body: the Revoke button's
  // ref was detached before the dialog looked for it, and the inline form
  // unmounted with nothing to take focus.
  describe("where focus goes afterwards", () => {
    beforeEach(() => {
      getDevicesMock.mockResolvedValue({
        error: false,
        data: { devices: [device(), device({ id: 2, name: "Bedroom TV" })] },
      });
    });

    it("returns to Revoke when the confirmation is cancelled", async () => {
      const user = userEvent.setup();
      renderCard();

      const revoke = await screen.findByRole("button", {
        name: "Revoke Living Room TV",
      });
      await user.click(revoke);
      await screen.findByRole("alertdialog");
      await user.click(screen.getByRole("button", { name: "Cancel" }));

      await waitFor(() => expect(revoke).toHaveFocus());
    });

    it("moves to the Devices heading once the device is gone", async () => {
      revokeDeviceMock.mockResolvedValue({ error: false, message: "Device revoked" });
      const user = userEvent.setup();
      renderCard();

      await user.click(
        await screen.findByRole("button", { name: "Revoke Living Room TV" }),
      );
      await screen.findByRole("alertdialog");
      // From here the server no longer lists the device.
      getDevicesMock.mockResolvedValue({
        error: false,
        data: { devices: [device({ id: 2, name: "Bedroom TV" })] },
      });
      await user.click(screen.getByRole("button", { name: "Revoke" }));

      await waitFor(() =>
        expect(screen.getByRole("heading", { name: "Devices" })).toHaveFocus(),
      );
      // The row left with the save, not with the list's next refetch.
      expect(
        screen.queryByRole("button", { name: "Revoke Living Room TV" }),
      ).toBeNull();
    });

    it("returns to Rename when the inline form is cancelled", async () => {
      const user = userEvent.setup();
      renderCard();

      await user.click(
        await screen.findByRole("button", { name: "Rename Living Room TV" }),
      );
      await user.click(screen.getByRole("button", { name: "Cancel" }));

      await waitFor(() =>
        expect(
          screen.getByRole("button", { name: "Rename Living Room TV" }),
        ).toHaveFocus(),
      );
    });

    it("cancels the rename on Escape and returns to Rename", async () => {
      const user = userEvent.setup();
      renderCard();

      await user.click(
        await screen.findByRole("button", { name: "Rename Living Room TV" }),
      );
      await user.type(
        screen.getByRole("textbox", { name: "New name for Living Room TV" }),
        " 2{Escape}",
      );

      expect(screen.queryByRole("textbox")).toBeNull();
      expect(renameDeviceMock).not.toHaveBeenCalled();
      await waitFor(() =>
        expect(
          screen.getByRole("button", { name: "Rename Living Room TV" }),
        ).toHaveFocus(),
      );
    });

    it("returns to Rename, under its new name, after a save", async () => {
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
      getDevicesMock.mockResolvedValue({
        error: false,
        data: {
          devices: [
            device({ name: "Den TV" }),
            device({ id: 2, name: "Bedroom TV" }),
          ],
        },
      });
      await user.type(input, "Den TV{Enter}");

      await waitFor(() =>
        expect(
          screen.getByRole("button", { name: "Rename Den TV" }),
        ).toHaveFocus(),
      );
    });
  });

  it("words list errors itself instead of echoing the client's canned text", async () => {
    getDevicesMock.mockResolvedValue({
      error: true,
      message: "500 - A network error occurred while processing your request.",
      status: 500,
    });

    renderCard();

    const error = await screen.findByRole("alert");
    expect(error).toHaveTextContent("Couldn’t load your devices. Please try again.");
    expect(error).not.toHaveTextContent(/network error/);
  });
});
