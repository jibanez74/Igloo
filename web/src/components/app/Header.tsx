import { useState } from "react";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { Search } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import NotificationBell from "@/components/app/NotificationBell";
import ThemeToggle from "@/components/app/ThemeToggle";
import { inputIconClassName, lightInputClassName } from "@/lib/input-styles";

export default function Header() {
  const navigate = useNavigate();
  const search = useSearch({ strict: false });
  const [value, setValue] = useState(search.q ?? "");
  const [prevQ, setPrevQ] = useState(search.q);

  // Sync the input to the URL's q param when it changes (e.g. browser
  // back/forward) by adjusting state during render — see React's
  // "You Might Not Need an Effect".
  if (search.q !== prevQ) {
    setPrevQ(search.q);
    setValue(search.q ?? "");
  }

  const handleSubmit = (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    // Dismiss the mobile on-screen keyboard by removing focus from the input.
    const active = document.activeElement;
    if (active instanceof HTMLElement) active.blur();

    const q = value.trim();
    navigate({
      to: "/search",
      search: { q, tab: "all", page: 1 },
    });
  };

  return (
    <>
      {/* Search */}
      <form
        className="min-w-0 flex-1 sm:max-w-lg"
        role="search"
        aria-label="Search library"
        onSubmit={handleSubmit}
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
            value={value}
            onChange={(e) => setValue(e.target.value)}
            className={`pl-10 ${lightInputClassName}`}
          />
        </div>
      </form>

      {/* Utility buttons */}
      <nav className="ml-auto flex shrink-0 items-center gap-1">
        <NotificationBell />
        <ThemeToggle />
      </nav>
    </>
  );
}
