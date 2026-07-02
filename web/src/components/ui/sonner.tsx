import { useSyncExternalStore } from "react";
import { Toaster as Sonner, type ToasterProps } from "sonner";
import { getStoredTheme, subscribeTheme } from "@/lib/theme";

export function Toaster(props: ToasterProps) {
  const theme = useSyncExternalStore(subscribeTheme, getStoredTheme);

  return (
    <Sonner
      theme={theme}
      closeButton
      position="top-right"
      richColors
      toastOptions={{
        classNames: {
          toast: "bg-muted border-border text-foreground shadow-xl",
          title: "text-foreground font-medium",
          description: "text-muted-foreground",
          closeButton:
            "bg-accent border-border text-muted-foreground hover:bg-muted hover:text-foreground",
          success: "bg-card border-success/50 text-card-foreground",
          error: "bg-card border-destructive/50 text-card-foreground",
          info: "bg-muted border-primary/30 text-foreground",
        },
      }}
      {...props}
    />
  );
}
