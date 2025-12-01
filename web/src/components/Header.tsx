import { Link } from "@tanstack/react-router";

export default function Header() {
  return (
    <header className="sticky top-0 z-50 bg-slate-900/80 backdrop-blur border-b border-slate-800">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 h-16 flex items-center gap-4">
        <Link
          to="/"
          className="flex items-center gap-2 font-semibold tracking-wide"
        >
          <i
            className="fa-solid fa-igloo text-amber-400 text-xl"
            aria-hidden="true"
          ></i>
          <span className="text-lg">Igloo</span>
        </Link>

        <form
          className="ml-auto flex-1 max-w-xl"
          role="search"
          aria-label="Search library"
        >
          <label className="sr-only" htmlFor="q">
            Search
          </label>
          <div className="relative">
            <i
              className="fa-solid fa-magnifying-glass absolute left-3 top-1/2 -translate-y-1/2 text-slate-400"
              aria-hidden="true"
            ></i>
            <input
              id="q"
              name="q"
              type="search"
              placeholder="Search..."
              className="w-full rounded-lg bg-slate-800/80 pl-10 pr-3 py-2 text-sm placeholder-slate-400
                   focus:outline-none focus:ring-2 focus:ring-amber-400"
            />
          </div>
        </form>

        <nav className="flex items-center gap-2 ml-2">
          <button
            className="p-2 rounded-lg hover:bg-slate-800 focus:outline-none focus:ring-2 focus:ring-amber-400"
            aria-label="Notifications"
          >
            <i className="fa-regular fa-bell" aria-hidden="true"></i>
          </button>

          <button
            className="p-2 rounded-lg hover:bg-slate-800 focus:outline-none focus:ring-2 focus:ring-amber-400"
            aria-label="Settings"
          >
            <i className="fa-solid fa-gear" aria-hidden="true"></i>
          </button>
        </nav>
      </div>
    </header>
  );
}
