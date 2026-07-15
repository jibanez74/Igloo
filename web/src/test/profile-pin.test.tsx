import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ProfilePinCard from "@/components/ProfilePinCard";
import type { AuthUser } from "@/types";

const getAuthUserMock = vi.fn();
const getUserPinMock = vi.fn();
const updateUserPinMock = vi.fn();
const showSuccessMock = vi.fn();
const showActionFailedMock = vi.fn();
const showValidationErrorMock = vi.fn();

vi.mock("@/lib/api", () => ({
  getAuthUser: (...args: unknown[]) => getAuthUserMock(...args),
  getUserPin: (...args: unknown[]) => getUserPinMock(...args),
  updateUserPin: (...args: unknown[]) => updateUserPinMock(...args),
}));

vi.mock("@/lib/toast-helpers", () => ({
  showSuccess: (...args: unknown[]) => showSuccessMock(...args),
  showActionFailed: (...args: unknown[]) => showActionFailedMock(...args),
  showValidationError: (...args: unknown[]) => showValidationErrorMock(...args),
}));

function testUser(overrides: Partial<AuthUser> = {}): AuthUser {
  return {
    id: 2,
    name: "Dana Scully",
    email: "dana@example.com",
    is_admin: false,
    avatar: null,
    has_pin: false,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

function authUserResponse(user: AuthUser) {
  return { error: false, data: { user } };
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
      <ProfilePinCard />
    </QueryClientProvider>,
  );

  return queryClient;
}

describe("ProfilePinCard", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    getAuthUserMock.mockResolvedValue(authUserResponse(testUser()));
    getUserPinMock.mockResolvedValue({
      error: false,
      data: { pin: "1234" },
    });
    updateUserPinMock.mockResolvedValue({
      error: false,
      message: "PIN updated successfully",
      data: { user: testUser({ has_pin: true }) },
    });
  });

  it("sets a first PIN and flips to the has-PIN state", async () => {
    const user = userEvent.setup();
    renderCard();

    expect(
      await screen.findByText("You have not set a profile PIN."),
    ).toBeInTheDocument();

    await user.type(screen.getByLabelText("PIN"), "1234");
    await user.click(screen.getByRole("button", { name: "Set PIN" }));

    await waitFor(() => {
      expect(updateUserPinMock).toHaveBeenCalledWith("1234", undefined);
    });
    expect(showSuccessMock).toHaveBeenCalledWith("PIN saved");

    // The cache was patched with the returned user, so the card now offers
    // change/remove instead of first-time setup.
    expect(
      await screen.findByRole("button", { name: "Show PIN" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Update PIN" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Remove PIN" }),
    ).toBeInTheDocument();
  });

  it("exposes invalid PIN validation to assistive technology", async () => {
    const user = userEvent.setup();
    renderCard();

    const pinInput = await screen.findByLabelText("PIN");
    await user.type(pinInput, "12");
    await user.click(screen.getByRole("button", { name: "Set PIN" }));

    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent("PIN must be exactly 4 digits.");
    expect(pinInput).toHaveAttribute("aria-invalid", "true");
    expect(pinInput).toHaveAttribute(
      "aria-describedby",
      expect.stringContaining(alert.id),
    );
    expect(showValidationErrorMock).toHaveBeenCalledWith(
      "PIN must be exactly 4 digits.",
    );
    expect(updateUserPinMock).not.toHaveBeenCalled();
  });

  it("requires the current PIN before changing an existing one", async () => {
    getAuthUserMock.mockResolvedValue(
      authUserResponse(testUser({ has_pin: true })),
    );
    const user = userEvent.setup();
    renderCard();

    await user.type(await screen.findByLabelText("New PIN"), "5678");
    await user.click(screen.getByRole("button", { name: "Update PIN" }));

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Current PIN is required.",
    );
    expect(updateUserPinMock).not.toHaveBeenCalled();

    await user.type(screen.getByLabelText("Current PIN"), "1234");
    await user.click(screen.getByRole("button", { name: "Update PIN" }));

    await waitFor(() => {
      expect(updateUserPinMock).toHaveBeenCalledWith("5678", "1234");
    });
  });

  it("reveals and hides the current PIN on demand", async () => {
    getAuthUserMock.mockResolvedValue(
      authUserResponse(testUser({ has_pin: true })),
    );
    const user = userEvent.setup();
    renderCard();

    const toggle = await screen.findByRole("button", { name: "Show PIN" });
    expect(toggle).toHaveAttribute("aria-pressed", "false");
    expect(getUserPinMock).not.toHaveBeenCalled();
    expect(screen.getByText("••••")).toBeInTheDocument();

    await user.click(toggle);

    expect(await screen.findByText("1234")).toBeInTheDocument();
    expect(getUserPinMock).toHaveBeenCalledTimes(1);
    const hideToggle = screen.getByRole("button", { name: "Hide PIN" });
    expect(hideToggle).toHaveAttribute("aria-pressed", "true");

    await user.click(hideToggle);

    expect(screen.getByText("••••")).toBeInTheDocument();
    expect(screen.queryByText("1234")).not.toBeInTheDocument();
  });

  it("removes the PIN with the current PIN", async () => {
    getAuthUserMock.mockResolvedValue(
      authUserResponse(testUser({ has_pin: true })),
    );
    updateUserPinMock.mockResolvedValue({
      error: false,
      message: "PIN removed successfully",
      data: { user: testUser({ has_pin: false }) },
    });
    const user = userEvent.setup();
    renderCard();

    await user.type(await screen.findByLabelText("Current PIN"), "1234");
    await user.click(screen.getByRole("button", { name: "Remove PIN" }));

    await waitFor(() => {
      expect(updateUserPinMock).toHaveBeenCalledWith("", "1234");
    });
    expect(showSuccessMock).toHaveBeenCalledWith("PIN removed");
    expect(
      await screen.findByText("You have not set a profile PIN."),
    ).toBeInTheDocument();
  });

  it("surfaces server rejections on the current PIN field", async () => {
    getAuthUserMock.mockResolvedValue(
      authUserResponse(testUser({ has_pin: true })),
    );
    updateUserPinMock.mockResolvedValue({
      error: true,
      message: "current PIN is incorrect",
      status: 401,
    });
    const user = userEvent.setup();
    renderCard();

    await user.type(await screen.findByLabelText("Current PIN"), "0000");
    await user.type(screen.getByLabelText("New PIN"), "5678");
    await user.click(screen.getByRole("button", { name: "Update PIN" }));

    await waitFor(() => {
      expect(showActionFailedMock).toHaveBeenCalledWith(
        "update PIN",
        "current PIN is incorrect",
      );
    });
    expect(screen.getByRole("alert")).toHaveTextContent(
      "current PIN is incorrect",
    );
    expect(screen.getByLabelText("Current PIN")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
  });

  it("surfaces initial setup rejections on the visible PIN field", async () => {
    updateUserPinMock.mockResolvedValue({
      error: true,
      message: "PIN is not allowed",
      status: 400,
    });
    const user = userEvent.setup();
    renderCard();

    const pinInput = await screen.findByLabelText("PIN");
    await user.type(pinInput, "1234");
    await user.click(screen.getByRole("button", { name: "Set PIN" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "PIN is not allowed",
    );
    expect(pinInput).toHaveAttribute("aria-invalid", "true");
    expect(pinInput).toHaveFocus();
  });

  it("surfaces thrown initial setup errors on the visible PIN field", async () => {
    updateUserPinMock.mockRejectedValue(new Error("network failure"));
    const user = userEvent.setup();
    renderCard();

    const pinInput = await screen.findByLabelText("PIN");
    await user.type(pinInput, "1234");
    await user.click(screen.getByRole("button", { name: "Set PIN" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "An unexpected error occurred",
    );
    expect(pinInput).toHaveAttribute("aria-invalid", "true");
    expect(pinInput).toHaveFocus();
  });
});
