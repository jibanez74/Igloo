import type { PropsWithChildren } from "react";
import { useMatches } from "@tanstack/react-router";
import Header from "@/components/app/Header";
import AppSidebar from "@/components/app/app-sidebar";
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar";
import { useIsMiniPlayerVisible } from "@/hooks/useIsMiniPlayerVisible";
import { MINI_PLAYER_CLEARANCE_PADDING_CLASS } from "@/lib/constants";
import { cn } from "@/lib/utils";

function handleSkipToContent() {
  requestAnimationFrame(() => {
    document.getElementById("main")?.focus();
  });
}

export default function AppShell({ children }: PropsWithChildren) {
  // The minimized audio player is a fixed bottom bar; reserve space for it so
  // the last rows of any page stay reachable while music plays.
  const isMiniPlayerVisible = useIsMiniPlayerVisible();
  // A player route asks for the whole window (`staticData.immersive`): the
  // sidebar, the header and the page padding step aside, and the page keeps
  // the one main landmark.
  const immersive = useMatches({
    select: matches => matches.some(match => match.staticData.immersive === true),
  });

  if (immersive) {
    return (
      <main
        id="main"
        tabIndex={-1}
        data-immersive="true"
        // At least the window, and taller when the page is: a player fills
        // it exactly, and a longer page (a watch room on a phone) scrolls.
        className="flex min-h-svh min-w-0 flex-col bg-background focus:outline-hidden"
      >
        {children}
      </main>
    );
  }

  return (
    <SidebarProvider>
      <a
        href="#main"
        onClick={handleSkipToContent}
        className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-modal focus:rounded-md focus:bg-primary focus:px-3 focus:py-2 focus:text-primary-foreground"
      >
        Skip to page content
      </a>

      <AppSidebar />

      <SidebarInset className="bg-background">
        <header className="sticky top-0 z-chrome flex h-14 shrink-0 items-center gap-4 border-b border-border bg-background/95 px-4 backdrop-blur-sm md:px-6">
          <SidebarTrigger className="-ml-2" />
          <Header />
        </header>

        {/* The header above is the banner; only page content is main, so
            "Skip to page content" lands past the search bar. */}
        <main
          id="main"
          tabIndex={-1}
          className={cn(
            // overflow-x-clip (not hidden/auto): clipping must not create a
            // scroll container, or sticky elements inside pages stop sticking
            // to the window scroll.
            "flex min-h-0 min-w-0 flex-1 flex-col overflow-x-clip px-4 py-6 focus:outline-hidden sm:px-6 lg:px-8",
            isMiniPlayerVisible && MINI_PLAYER_CLEARANCE_PADDING_CLASS,
          )}
        >
          {children}
        </main>
      </SidebarInset>
    </SidebarProvider>
  );
}
