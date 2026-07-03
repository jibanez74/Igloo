import { type PropsWithChildren } from "react";
import Header from "@/components/Header";
import AppSidebar from "@/components/app-sidebar";
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar";

export default function AppShell({ children }: PropsWithChildren) {
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

        <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-x-hidden overflow-y-auto px-4 py-6 sm:px-6 lg:px-8">
          {children}
        </div>
      </SidebarInset>
    </SidebarProvider>
  );
}
