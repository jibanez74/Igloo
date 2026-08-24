import { SELECT_CONTENT_SLOT_SELECTOR } from "@/lib/constants";

function isTargetInsideRadixSelectContent(
  target: EventTarget | null,
): boolean {
  return (
    target instanceof Element &&
    target.closest(SELECT_CONTENT_SLOT_SELECTOR) !== null
  );
}

export function preventDialogDismissIfRadixSelectContent(
  event: { preventDefault: () => void; detail: { originalEvent: Event } },
): void {
  const original = event.detail.originalEvent;
  const target =
    "target" in original && original.target != null
      ? original.target
      : null;
  if (isTargetInsideRadixSelectContent(target)) {
    event.preventDefault();
  }
}
