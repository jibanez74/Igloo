import { createContext, useContext } from "react";

export const AppShellScrollContainerContext = createContext<HTMLElement | null>(
  null,
);

export function useAppShellScrollContainer() {
  return useContext(AppShellScrollContainerContext);
}
