import { Search, Bell, Cast } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { inputIconClassName, lightInputClassName } from "@/lib/input-styles";

export default function Header() {
  return (
    <>
      {/* Search */}
      <form
        className="min-w-0 flex-1 sm:max-w-lg"
        role="search"
        aria-label="Search library"
      >
        <Label htmlFor="q" className="sr-only">
          Search
        </Label>
        <div className="relative">
          <Search
            className={inputIconClassName}
            aria-hidden="true"
          />
          <Input
            id="q"
            name="q"
            type="search"
            placeholder="Search..."
            className={`pl-10 ${lightInputClassName}`}
          />
        </div>
      </form>

      {/* Utility buttons */}
      <nav className="ml-auto flex shrink-0 items-center gap-1">
        <Button
          variant="ghost"
          size="icon"
          aria-label="Notifications"
          className="text-slate-300 hover:bg-slate-800 hover:text-white"
        >
          <Bell aria-hidden="true" />
        </Button>

        <Button
          variant="ghost"
          size="icon"
          aria-label="Cast"
          className="text-slate-300 hover:bg-slate-800 hover:text-white"
        >
          <Cast aria-hidden="true" />
        </Button>
      </nav>
    </>
  );
}
