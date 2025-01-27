import { useState, useEffect, useRef } from "react";
import { Link } from "@tanstack/react-router";
import {
  FiHome,
  FiFilm,
  FiTv,
  FiMusic,
  FiGrid,
  FiMenu,
  FiX,
} from "react-icons/fi";
import iglooLogo from "@/assets/images/logo-alt.png";

const navLinks = [
  { to: "/", icon: FiHome, label: "Home" },
  { to: "/movies", icon: FiFilm, label: "Movies" },
  { to: "/tvshows", icon: FiTv, label: "TV Shows" },
  { to: "/music", icon: FiMusic, label: "Music" },
  { to: "/others", icon: FiGrid, label: "Others" },
];

export default function Navbar() {
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const mobileMenuRef = useRef<HTMLDivElement>(null);
  const menuButtonRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (
        isMobileMenuOpen &&
        mobileMenuRef.current &&
        !mobileMenuRef.current.contains(event.target as Node) &&
        menuButtonRef.current &&
        !menuButtonRef.current.contains(event.target as Node)
      ) {
        setIsMobileMenuOpen(false);
      }
    };

    document.addEventListener("mousedown", handleClickOutside);

    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [isMobileMenuOpen]);

  const toggleMobileMenu = () => setIsMobileMenuOpen(!isMobileMenuOpen);

  return (
    <>
      {/* Fade Overlay */}
      <div
        className={`fixed inset-0 bg-gradient-to-b from-slate-950/80 to-blue-950/80 backdrop-blur-sm transition-all duration-300 md:hidden ${
          isMobileMenuOpen ? "opacity-100" : "opacity-0 pointer-events-none"
        }`}
        onClick={() => setIsMobileMenuOpen(false)}
        aria-hidden='true'
      />

      <nav className='fixed top-0 left-0 right-0 bg-slate-900/95 shadow-lg shadow-blue-900/20 backdrop-blur-sm z-50'>
        <div className='max-w-7xl mx-auto px-4 sm:px-6 lg:px-8'>
          <div className='flex items-center justify-between h-16'>
            {/* Left side - Logo and Navigation */}
            <div className='flex items-center gap-8'>
              <Link
                to='/'
                className='flex items-center gap-2 text-white hover:text-blue-300 transition-colors'
                preload='intent'
              >
                <img src={iglooLogo} alt='Igloo' className='h-8 w-auto' />
                <span className='text-xl font-semibold bg-gradient-to-r from-blue-200 to-blue-100 text-transparent bg-clip-text'>
                  Igloo
                </span>
              </Link>

              {/* Desktop Navigation Links */}
              <div className='hidden md:flex items-center gap-1'>
                {navLinks.map(({ to, icon: Icon, label }) => (
                  <Link
                    key={to}
                    to={to}
                    preload='intent'
                    className='px-3 py-2 rounded-md text-sm font-medium text-blue-200 hover:text-white hover:bg-blue-500/10 transition-all duration-200 ease-in-out flex items-center gap-2 relative group'
                  >
                    <Icon className='w-4 h-4' aria-hidden='true' />
                    {label}
                    <span className='absolute inset-x-0 -bottom-px h-px bg-gradient-to-r from-transparent via-blue-500 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500' />
                  </Link>
                ))}
              </div>
            </div>

            {/* Right side - Settings and Mobile Menu Button */}
            <div className='flex items-center gap-2'>
              {/* Settings Link - Temporarily disabled until route is implemented
              <Link
                to='/settings'
                preload='intent'
                className='p-2 rounded-md text-blue-200 hover:text-white hover:bg-blue-500/10 transition-all duration-200 ease-in-out flex items-center gap-2 relative group'
                title='Settings'
              >
                <FiSettings className='w-5 h-5' aria-hidden='true' />
                <span className='hidden md:inline text-sm font-medium'>
                  Settings
                </span>
                <span className='absolute inset-x-0 -bottom-px h-px bg-gradient-to-r from-transparent via-blue-500 to-transparent opacity-0 group-hover:opacity-100 transition-opacity duration-500' />
              </Link>
              */}

              {/* Mobile Menu Button */}
              <button
                ref={menuButtonRef}
                type='button'
                className='md:hidden p-2 rounded-md text-blue-200 hover:text-white hover:bg-blue-500/10 transition-colors'
                onClick={toggleMobileMenu}
                aria-expanded={isMobileMenuOpen}
                aria-label='Toggle mobile menu'
              >
                {isMobileMenuOpen ? (
                  <FiX className='w-6 h-6' aria-hidden='true' />
                ) : (
                  <FiMenu className='w-6 h-6' aria-hidden='true' />
                )}
              </button>
            </div>
          </div>
        </div>

        {/* Mobile Navigation Menu */}
        <div
          ref={mobileMenuRef}
          className={`md:hidden transition-all duration-300 ease-in-out ${
            isMobileMenuOpen
              ? "max-h-96 opacity-100"
              : "max-h-0 opacity-0 pointer-events-none"
          }`}
        >
          <div className='px-2 pt-2 pb-3 space-y-1 bg-slate-900/95 backdrop-blur-sm border-t border-blue-900/20'>
            {navLinks.map(({ to, icon: Icon, label }) => (
              <Link
                key={to}
                to={to}
                preload='intent'
                className='px-3 py-2 rounded-md text-base font-medium text-blue-200 hover:text-white hover:bg-blue-500/10 transition-colors flex items-center gap-3'
                onClick={() => setIsMobileMenuOpen(false)}
              >
                <Icon className='w-5 h-5' aria-hidden='true' />
                {label}
              </Link>
            ))}
          </div>
        </div>
      </nav>
    </>
  );
}
