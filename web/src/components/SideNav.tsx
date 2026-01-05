import { Link, type LinkProps } from "@tanstack/react-router";

type NavItem = {
  to: LinkProps["to"];
  icon: string;
  label: string;
  exact?: boolean;
};

const navItems: NavItem[] = [
  { to: "/", icon: "fa-house", label: "Home", exact: true },
  { to: "/music", icon: "fa-music", label: "Music" },
];

export default function SideNav() {
  return (
    <aside className='md:sticky md:top-16 md:h-[calc(100vh-8rem)] md:overflow-y-auto'>
      <nav aria-labelledby='primary-nav' className='md:pr-4'>
        <h2 id='primary-nav' className='sr-only'>
          Primary navigation
        </h2>
        <ul className='flex md:flex-col gap-2 overflow-x-auto md:overflow-visible'>
          {navItems.map(item => (
            <li key={String(item.to)}>
              <Link
                to={item.to}
                activeOptions={{ exact: item.exact }}
                className='flex items-center gap-3 rounded-lg px-3 py-2 text-slate-300
                        hover:text-white hover:bg-slate-800 focus:outline-none focus:ring-2 focus:ring-amber-400
                          md:border-l-2 md:border-transparent transition-colors'
                activeProps={{
                  className:
                    "flex items-center gap-3 rounded-lg px-3 py-2 text-white bg-slate-800 font-medium focus:outline-none focus:ring-2 focus:ring-amber-400 md:border-l-2 md:border-amber-400 transition-colors",
                }}
              >
                {({ isActive }) => (
                  <>
                    <i
                      className={`fa-solid ${item.icon} ${isActive ? "text-amber-400" : "text-slate-400"}`}
                      aria-hidden='true'
                    />
                    <span>{item.label}</span>
                  </>
                )}
              </Link>
          </li>
          ))}
        </ul>
      </nav>
    </aside>
  );
}
