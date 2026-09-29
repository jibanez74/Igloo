import { useMediaQuery } from "@/hooks/useMediaQuery";

export function usePrefersCoarsePointer() {
  return useMediaQuery("(hover: none) and (pointer: coarse)");
}
