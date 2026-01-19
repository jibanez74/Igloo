import { Link } from "@tanstack/react-router";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";

export default function Header() {
  return (
    <header className="sticky top-0 z-50 border-b border-slate-800 bg-slate-900/80 backdrop-blur-sm">
      <div className="mx-auto flex h-16 max-w-7xl items-center gap-4 px-4 sm:px-6 lg:px-8">
        <Link
          to="/"
          className="flex items-center gap-2 font-semibold tracking-wide"
        >
          <i
            className="fa-solid fa-igloo text-xl text-amber-400"
            aria-hidden="true"
          />
          <span className="text-lg">Igloo</span>
        </Link>

        <form
          className="ml-auto max-w-xl flex-1"
          role="search"
          aria-label="Search library"
        >
          <Label htmlFor="q" className="sr-only">
            Search
          </Label>
          <div className="relative">
            <i
              className="fa-solid fa-magnifying-glass absolute top-1/2 left-3 z-10 -translate-y-1/2 text-slate-400"
              aria-hidden="true"
            />
            <Input
              id="q"
              name="q"
              type="search"
              placeholder="Search..."
              className="pl-10"
            />
          </div>
        </form>

        <nav className="ml-2 flex items-center gap-1">
          <Button variant="ghost" size="icon" aria-label="Notifications">
            <i className="fa-regular fa-bell" aria-hidden="true" />
          </Button>

          <Button variant="ghost" size="icon" aria-label="Settings">
            <i className="fa-solid fa-gear" aria-hidden="true" />
          </Button>
                  </nav>
      </div>
    </header>
  );
}
