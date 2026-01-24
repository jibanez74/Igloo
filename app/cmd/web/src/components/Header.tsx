import { Search, Bell, Cast } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export default function Header() {
  return (
    <>
      {/* Search */}
      <form
        className="max-w-xl flex-1"
        role="search"
        aria-label="Search library"
      >
        <Label htmlFor="q" className="sr-only">
          Search
        </Label>
        <div className="relative">
          <Search
            className="absolute top-1/2 left-3 z-10 size-4 -translate-y-1/2 text-slate-400"
            aria-hidden="true"
          />
          <Input
            id="q"
            name="q"
            type="search"
            placeholder="Search..."
            className="border-slate-700 bg-slate-800/50 pl-10 text-white placeholder:text-slate-400 focus:border-amber-500/50 focus:ring-amber-500/20"
          />
        </div>
      </form>

      {/* Utility buttons */}
      <nav className="ml-auto flex items-center gap-1">
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
