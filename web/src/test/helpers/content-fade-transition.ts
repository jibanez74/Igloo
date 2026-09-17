import { act } from "@testing-library/react";
import { expect, type MockInstance } from "vitest";
import { CONTENT_FADE_TRANSITION_MS } from "@/lib/constants";

export async function runContentFadeTransitionTimeout(
  setTimeoutSpy: MockInstance<typeof window.setTimeout>,
) {
  const transitionCall = setTimeoutSpy.mock.calls.find(
    ([, delay]) => delay === CONTENT_FADE_TRANSITION_MS,
  );
  const transitionCallback = transitionCall?.[0];

  expect(transitionCall).toBeDefined();

  if (typeof transitionCallback !== "function") {
    throw new Error("Expected content fade transition timeout callback.");
  }

  await act(async () => {
    await transitionCallback();
  });
}
