import { observeElementRect, type Rect, type Virtualizer } from "@tanstack/react-virtual";

export function getOffsetWithinScrollContainer(
  element: HTMLElement,
  scrollContainer: HTMLElement,
) {
  const elementRect = element.getBoundingClientRect();
  const scrollContainerRect = scrollContainer.getBoundingClientRect();

  return elementRect.top - scrollContainerRect.top + scrollContainer.scrollTop;
}

export function observeElementRectWithWindowFallback<
  TScrollElement extends Element,
  TItemElement extends Element,
>(
  instance: Virtualizer<TScrollElement, TItemElement>,
  cb: (rect: Rect) => void,
) {
  return observeElementRect(instance, rect => {
    if (rect.width === 0 && rect.height === 0) {
      cb({ width: window.innerWidth, height: window.innerHeight });
      return;
    }

    cb(rect);
  });
}
