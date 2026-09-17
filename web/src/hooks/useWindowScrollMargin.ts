import { useEffect, useRef, useState } from "react";

/**
 * Tracks a list element's distance from the top of the document so a
 * `useWindowVirtualizer` can map window scroll onto item indices. The Igloo app
 * shell scrolls the window (not an inner container), so window-virtualized lists
 * must offset their scrollMargin by the element's document-relative top.
 */
export function useWindowScrollMargin<T extends HTMLElement = HTMLDivElement>() {
  const listRef = useRef<T>(null);
  const [scrollMargin, setScrollMargin] = useState(0);

  useEffect(() => {
    const listElement = listRef.current;
    if (!listElement) {
      return;
    }

    const updateScrollMargin = () => {
      setScrollMargin(listElement.getBoundingClientRect().top + window.scrollY);
    };

    updateScrollMargin();

    const resizeObserver =
      typeof ResizeObserver === "undefined"
        ? null
        : new ResizeObserver(updateScrollMargin);

    resizeObserver?.observe(listElement);
    if (typeof document !== "undefined") {
      resizeObserver?.observe(document.body);
    }
    window.addEventListener("resize", updateScrollMargin);

    return () => {
      resizeObserver?.disconnect();
      window.removeEventListener("resize", updateScrollMargin);
    };
  }, []);

  return { listRef, scrollMargin };
}
