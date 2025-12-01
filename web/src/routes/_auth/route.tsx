import { Link, Outlet, createFileRoute } from "@tanstack/react-router";
// import { authUserQueryOpts } from "@/lib/query-opts";

export const Route = createFileRoute("/_auth")({
  // beforeLoad: async ({ context, location }) => {
  //   const res = await context.queryClient.ensureQueryData(authUserQueryOpts());

  //   if (res.error) {
  //     throw redirect({
  //       to: "/login",
  //       from: location.href,
  //     });
  //   }
  // },
  component: AuthLayout,
});

function AuthLayout() {
  return (
    <>
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3
         focus:bg-amber-400 focus:text-slate-900 focus:px-3 focus:py-2 focus:rounded-md"
      >
        Skip to content
      </a>

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

      <div className="flex-1 flex overflow-hidden">
        <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-6 grid md:grid-cols-[14rem_1fr] gap-6 flex-1">
          <aside className="md:sticky md:top-16 md:h-[calc(100vh-8rem)] md:overflow-y-auto">
            <nav aria-labelledby="primary-nav" className="md:pr-4">
              <h2 id="primary-nav" className="sr-only">
                Primary navigation
              </h2>
              <ul className="flex md:flex-col gap-2 overflow-x-auto md:overflow-visible">
                <li>
                  <a
                    href="/"
                    data-route="/"
                    className="flex items-center gap-3 rounded-lg px-3 py-2 text-slate-300
                        hover:text-white hover:bg-slate-800 focus:outline-none focus:ring-2 focus:ring-amber-400
                        md:border-l-2 md:border-transparent"
                  >
                    <i className="fa-solid fa-house text-slate-400"></i>
                    <span>Home</span>
                  </a>
                </li>

                <li>
                  <a
                    href="/movies"
                    data-route="/movies"
                    className="flex items-center gap-3 rounded-lg px-3 py-2 text-slate-300
                        hover:text-white hover:bg-slate-800 focus:outline-none focus:ring-2 focus:ring-amber-400
                        md:border-l-2 md:border-transparent"
                  >
                    <i className="fa-solid fa-film text-slate-400"></i>
                    <span>Movies</span>
                  </a>
                </li>

                <li>
                  <a
                    href="/tv-shows"
                    data-route="/tv-shows"
                    className="flex items-center gap-3 rounded-lg px-3 py-2 text-slate-300
                        hover:text-white hover:bg-slate-800 focus:outline-none focus:ring-2 focus:ring-amber-400
                        md:border-l-2 md:border-transparent"
                  >
                    <i className="fa-solid fa-tv text-slate-400"></i>
                    <span>TV Shows</span>
                  </a>
                </li>

                <li>
                  <a
                    href="/music"
                    data-route="/music"
                    className="flex items-center gap-3 rounded-lg px-3 py-2 text-slate-300
                        hover:text-white hover:bg-slate-800 focus:outline-none focus:ring-2 focus:ring-amber-400
                        md:border-l-2 md:border-transparent"
                  >
                    <i className="fa-solid fa-music text-slate-400"></i>
                    <span>Music</span>
                  </a>
                </li>

                <li>
                  <a
                    href="/photos"
                    data-route="/photos"
                    className="flex items-center gap-3 rounded-lg px-3 py-2 text-slate-300
                        hover:text-white hover:bg-slate-800 focus:outline-none focus:ring-2 focus:ring-amber-400
                        md:border-l-2 md:border-transparent"
                  >
                    <i className="fa-solid fa-image text-slate-400"></i>
                    <span>Photos</span>
                  </a>
                </li>

                <li>
                  <a
                    href="/other"
                    data-route="/other"
                    className="flex items-center gap-3 rounded-lg px-3 py-2 text-slate-300
                        hover:text-white hover:bg-slate-800 focus:outline-none focus:ring-2 focus:ring-amber-400
                        md:border-l-2 md:border-transparent"
                  >
                    <i className="fa-solid fa-ellipsis text-slate-400"></i>
                    <span>Other</span>
                  </a>
                </li>
              </ul>
            </nav>
          </aside>

          <main
            id="main"
            className="min-w-0 md:h-[calc(100vh-8rem)] md:overflow-y-auto"
          >
            <Outlet />
          </main>
        </div>
      </div>
    </>
  );
}
