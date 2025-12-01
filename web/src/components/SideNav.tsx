export default function SideNav() {
  return (
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
  );
}
