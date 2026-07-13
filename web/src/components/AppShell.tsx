import type { PropsWithChildren } from "react";
import Header from "@/components/Header";
import AppSidebar from "@/components/app-sidebar";
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar";
import { useIsMiniPlayerVisible } from "@/hooks/useIsMiniPlayerVisible";
import { MINI_PLAYER_CLEARANCE_PADDING_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";

export default function AppShell({ children }: PropsWithChildren) {
  // The minimized audio player is a fixed bottom bar; reserve space for it so
  // the last rows of any page stay reachable while music plays.
  const isMiniPlayerVisible = useIsMiniPlayerVisible();

  const handleSkipToContent = () => {
    requestAnimationFrame(() => {
      document.getElementById("main")?.focus();
    });
  };

  return (
    <SidebarProvider>
      <a
        href="#main"
        onClick={handleSkipToContent}
        className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-50 focus:rounded-md focus:bg-primary focus:px-3 focus:py-2 focus:text-primary-foreground"
      >
        Skip to content
      </a>

      <AppSidebar />

      <SidebarInset
        id="main"
        tabIndex={-1}
        className="bg-background focus:outline-none"
      >
        <header className="sticky top-0 z-40 flex h-14 shrink-0 items-center gap-4 border-b border-border bg-background/95 px-4 backdrop-blur-sm md:px-6">
          <SidebarTrigger className="-ml-1 text-muted-foreground hover:bg-accent hover:text-foreground md:hidden" />
          <Header />
        </header>

        <div
          className={cn(
            // overflow-x-clip (not hidden/auto): clipping must not create a
            // scroll container, or sticky elements inside pages stop sticking
            // to the window scroll.
            "flex min-h-0 min-w-0 flex-1 flex-col overflow-x-clip px-4 py-6 sm:px-6 lg:px-8",
            isMiniPlayerVisible && MINI_PLAYER_CLEARANCE_PADDING_CLASS,
          )}
        >
          {children}
        </div>
      </SidebarInset>
    </SidebarProvider>
  );
}
