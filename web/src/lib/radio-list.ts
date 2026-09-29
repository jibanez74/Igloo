import type { KeyboardEvent } from "react";

type RadioListKeyOptions<T> = {
  /** The id of the radio input that renders `item`, to move focus to it. */
  inputId: (item: T) => string;
  onSelect: (item: T) => void;
};

function isSpaceKey(event: KeyboardEvent) {
  return (
    event.key === " " ||
    event.key === "Space" ||
    event.key === "Spacebar" ||
    event.code === "Space"
  );
}

/**
 * Keyboard handling for a result list of radio inputs whose selection the
 * component owns: Space selects the focused item, and the arrow keys move focus
 * and selection to the next or previous item, wrapping at either end.
 */
export function handleRadioListKey<T>(
  event: KeyboardEvent<HTMLInputElement>,
  items: readonly T[],
  currentIndex: number,
  { inputId, onSelect }: RadioListKeyOptions<T>,
) {
  if (isSpaceKey(event)) {
    event.preventDefault();
    onSelect(items[currentIndex]);
    return;
  }

  if (items.length < 2) return;

  let nextIndex: number;
  if (event.key === "ArrowDown" || event.key === "ArrowRight") {
    nextIndex = (currentIndex + 1) % items.length;
  } else if (event.key === "ArrowUp" || event.key === "ArrowLeft") {
    nextIndex = (currentIndex - 1 + items.length) % items.length;
  } else {
    return;
  }

  event.preventDefault();

  const nextItem = items[nextIndex];
  const nextInput = document.getElementById(inputId(nextItem));
  if (nextInput instanceof HTMLInputElement) {
    nextInput.focus();
  }

  onSelect(nextItem);
}
