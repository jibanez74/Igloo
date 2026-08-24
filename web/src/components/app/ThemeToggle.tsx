import { useSyncExternalStore } from "react";
import { Moon, Sun } from "lucide-react";
import { Button } from "@/components/ui/button";
import { getActiveTheme, setTheme, subscribeTheme } from "@/lib/theme";

export default function ThemeToggle() {
  const theme = useSyncExternalStore(subscribeTheme, getActiveTheme);
  const nextTheme = theme === "dark" ? "light" : "dark";
  const label =
    nextTheme === "light" ? "Switch to light theme" : "Switch to dark theme";
  const Icon = nextTheme === "light" ? Sun : Moon;

  return (
    <Button
      variant="ghost"
      size="icon"
      type="button"
      aria-label={label}
      title={label}
      className="text-muted-foreground hover:bg-muted hover:text-foreground"
      onClick={() => setTheme(nextTheme)}
    >
      <Icon aria-hidden="true" />
    </Button>
  );
}
