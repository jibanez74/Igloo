import { useEffect, useLayoutEffect, useReducer, useState } from "react";
import { flushSync } from "react-dom";
import {
  Virtualizer,
  elementScroll,
  observeElementOffset,
  type PartialKeys,
  type VirtualizerOptions,
} from "@tanstack/react-virtual";

const useIsomorphicLayoutEffect =
  typeof document === "undefined" ? useEffect : useLayoutEffect;

type ElementVirtualizerOptions<
  TScrollElement extends Element,
  TItemElement extends Element,
> = PartialKeys<
  VirtualizerOptions<TScrollElement, TItemElement>,
  "observeElementOffset" | "scrollToFn"
>;

export function useElementVirtualizer<
  TScrollElement extends Element,
  TItemElement extends Element = Element,
>(
  options: ElementVirtualizerOptions<TScrollElement, TItemElement>,
): Virtualizer<TScrollElement, TItemElement> {
  const rerender = useReducer((count: number) => count + 1, 0)[1];

  const resolvedOptions: VirtualizerOptions<TScrollElement, TItemElement> = {
    observeElementOffset,
    scrollToFn: elementScroll,
    ...options,
    onChange: (instance, sync) => {
      if (sync) {
        flushSync(rerender);
      } else {
        rerender();
      }
      options.onChange?.(instance, sync);
    },
  };

  const [virtualizer] = useState(
    () => new Virtualizer<TScrollElement, TItemElement>(resolvedOptions),
  );

  virtualizer.setOptions(resolvedOptions);

  useIsomorphicLayoutEffect(() => virtualizer._didMount(), [virtualizer]);
  useIsomorphicLayoutEffect(() => virtualizer._willUpdate());

  return virtualizer;
}
