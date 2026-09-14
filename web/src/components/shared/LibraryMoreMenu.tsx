import { useState, type ReactNode, type RefObject } from "react";
import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import { MoreHorizontal, RefreshCw } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Spinner } from "@/components/ui/spinner";
import {
  FOCUS_VISIBLE_RING_CLASS,
  LIBRARY_MENU_ITEM_CLASS,
  MOTION_MICRO_CONTROL_CLASS,
} from "@/lib/constants";
import {
  refreshLibraryWithToasts,
  type LibraryRefreshNoun,
} from "@/lib/library-refresh";
import { cn } from "@/lib/utils";

type LibraryMoreMenuProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Exposed so a dialog opened from the menu can return focus to the trigger. */
  triggerRef?: RefObject<HTMLButtonElement | null>;
  children: ReactNode;
};

// The "More options" dropdown beside a library page's stats. The page decides
// what goes in it; every page has at least RefreshLibraryMenuItem.
export default function LibraryMoreMenu({
  open,
  onOpenChange,
  triggerRef,
  children,
}: LibraryMoreMenuProps) {
  return (
    <DropdownMenu open={open} onOpenChange={onOpenChange}>
      <DropdownMenuTrigger
        ref={triggerRef}
        className={cn(
          "inline-flex items-center justify-center rounded-full p-2 text-muted-foreground hover:bg-muted hover:text-foreground",
          MOTION_MICRO_CONTROL_CLASS,
          FOCUS_VISIBLE_RING_CLASS,
        )}
        aria-label="More options"
      >
        <MoreHorizontal className="size-5" aria-hidden="true" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="border-border bg-muted">
        {children}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

type RefreshLibraryMenuItemProps = {
  refresh: (queryClient: QueryClient) => Promise<void>;
  libraryNoun: LibraryRefreshNoun;
  /** Called once the refresh settles, so the owning menu can close. */
  onSettled: () => void;
};

export function RefreshLibraryMenuItem({
  refresh,
  libraryNoun,
  onSettled,
}: RefreshLibraryMenuItemProps) {
  const queryClient = useQueryClient();
  const [refreshingLibrary, setRefreshingLibrary] = useState(false);

  const handleRefreshLibrary = async () => {
    if (refreshingLibrary) return;

    setRefreshingLibrary(true);
    await refreshLibraryWithToasts(queryClient, refresh, libraryNoun);
    // refreshLibraryWithToasts owns the try/catch and never throws, precisely so
    // this reset can be plain sequential code (see its doc comment).
    // react-doctor-disable-next-line react-doctor/no-loading-flag-reset-outside-finally
    setRefreshingLibrary(false);
    onSettled();
  };

  return (
    <DropdownMenuItem
      className={LIBRARY_MENU_ITEM_CLASS}
      disabled={refreshingLibrary}
      onSelect={event => {
        // Keep the menu open while the async refresh runs so the
        // spinner/disabled state stays perceivable; it closes when the
        // refresh settles (see handleRefreshLibrary).
        event.preventDefault();
        if (refreshingLibrary) return;
        void handleRefreshLibrary();
      }}
    >
      {refreshingLibrary ? (
        <Spinner className="mr-2 size-4 text-primary" />
      ) : (
        <RefreshCw className="mr-2 size-4" aria-hidden="true" />
      )}
      Refresh Library
    </DropdownMenuItem>
  );
}
