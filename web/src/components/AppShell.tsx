import { useState, type PropsWithChildren } from "react";
import Header from "@/components/Header";
import AppSidebar from "@/components/app-sidebar";
import { AppShellScrollContainerContext } from "@/hooks/useAppShellScrollContainer";
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar";

export default function AppShell({ children }: PropsWithChildren) {
  const [scrollContainer, setScrollContainer] = useState<HTMLDivElement | null>(
    null,
  );

  return (
    <SidebarProvider>
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-50 focus:rounded-md focus:bg-amber-400 focus:px-3 focus:py-2 focus:text-slate-900"
      >
        Skip to content
      </a>

      <AppSidebar />

      <SidebarInset id="main" className="bg-slate-900">
        <header className="sticky top-0 z-40 flex h-14 shrink-0 items-center gap-4 border-b border-slate-800/50 bg-slate-900/95 px-4 backdrop-blur-sm md:px-6">
          <SidebarTrigger className="-ml-1 text-slate-300 hover:bg-slate-800 hover:text-white md:hidden" />
          <Header />
        </header>

        <AppShellScrollContainerContext.Provider value={scrollContainer}>
          <div
            ref={setScrollContainer}
            className="flex min-h-0 min-w-0 flex-1 flex-col overflow-x-hidden overflow-y-auto px-4 py-6 sm:px-6 lg:px-8"
          >
            {children}
          </div>
        </AppShellScrollContainerContext.Provider>
      </SidebarInset>
    </SidebarProvider>
  );
}
