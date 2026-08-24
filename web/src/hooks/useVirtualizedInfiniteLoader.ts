import { useRef } from "react";
import type { Virtualizer } from "@tanstack/react-virtual";

type UseVirtualizedInfiniteLoaderOptions = {
  itemCount: number;
  hasNextPage: boolean;
  isFetchingNextPage: boolean;
  fetchNextPage: () => Promise<unknown>;
  scopeKey: string | number;
  loadAheadItems?: number;
};

type VirtualizedInfiniteLoaderOnChange = <
  TScrollElement extends Element | Window,
  TItemElement extends Element,
>(
  virtualizer: Virtualizer<TScrollElement, TItemElement>,
) => void;

const DEFAULT_LOAD_AHEAD_ITEMS = 10;

export function useVirtualizedInfiniteLoader({
  itemCount,
  hasNextPage,
  isFetchingNextPage,
  fetchNextPage,
  scopeKey,
  loadAheadItems = DEFAULT_LOAD_AHEAD_ITEMS,
}: UseVirtualizedInfiniteLoaderOptions): VirtualizedInfiniteLoaderOnChange {
  const isFetchingNextRef = useRef(false);
  const requestedPageKeyRef = useRef<string | null>(null);

  const requestNextPage = async (requestKey: string) => {
    isFetchingNextRef.current = true;
    let fetchFailed = false;

    try {
      await fetchNextPage();
    } catch {
      fetchFailed = true;
    }

    if (fetchFailed && requestedPageKeyRef.current === requestKey) {
      requestedPageKeyRef.current = null;
    }

    isFetchingNextRef.current = false;
  };

  return (virtualizer) => {
    if (!hasNextPage || isFetchingNextPage || isFetchingNextRef.current) {
      return;
    }

    const renderedVirtualItems = virtualizer.getVirtualItems();
    const lastItem = renderedVirtualItems[renderedVirtualItems.length - 1];

    if (!lastItem || lastItem.index < itemCount - loadAheadItems) {
      return;
    }

    const requestKey = `${scopeKey}:${itemCount}`;

    if (requestedPageKeyRef.current === requestKey) {
      return;
    }

    requestedPageKeyRef.current = requestKey;
    void requestNextPage(requestKey);
  };
}
