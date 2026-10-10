import { beforeEach, describe, expect, it, vi } from "vitest";

const toastError = vi.fn();

vi.mock("sonner", () => ({ toast: { error: (...args: unknown[]) => toastError(...args) } }));

const { showActionFailed } = await import("@/lib/toast-helpers");

describe("showActionFailed", () => {
  beforeEach(() => {
    toastError.mockClear();
  });

  it("keeps a server-worded client error as the description", () => {
    showActionFailed("create user", {
      error: true,
      message: "A user with that email already exists.",
      status: 409,
    });

    expect(toastError).toHaveBeenCalledWith("Failed to create user", {
      description: "A user with that email already exists.",
    });
  });

  it("drops a server error constant instead of showing it", () => {
    showActionFailed("save settings", { error: true, message: "internal_error", status: 500 });

    expect(toastError).toHaveBeenCalledWith("Failed to save settings", undefined);
  });

  it("drops the client's canned 404 text", () => {
    showActionFailed("delete playlist", {
      error: true,
      message: "404 - The resource you requested was not found.",
      status: 404,
    });

    expect(toastError).toHaveBeenCalledWith("Failed to delete playlist", undefined);
  });

  it("passes a caller-worded string through", () => {
    showActionFailed("play album", "Something went wrong. Please try again.");

    expect(toastError).toHaveBeenCalledWith("Failed to play album", {
      description: "Something went wrong. Please try again.",
    });
  });
});
