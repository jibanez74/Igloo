import { useState } from "react";
import { NavLink, Link } from "react-router-dom";
import logo from "../assets/images/igloo-icon.png";
import profileDefault from "../assets/images/profile.png";
import { FaUser } from "react-icons/fa";

export default function Navbar() {
  const user = null;

  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [isProfileMenuOpen, setIsProfileMenuOpen] = useState(false);

  const closeMobileMenu = () => {
    setIsMobileMenuOpen(false);
  };

  return (
    <nav className='bg-blue-700 border-b border-blue-500'>
      <div className='mx-auto max-w-7xl px-2 sm:px-6 lg:px-8'>
        <div className='relative flex h-20 items-center justify-between'>
          <div className='absolute inset-y-0 left-0 flex items-center sm:hidden'>
            {/* Mobile menu button */}
            <button
              type='button'
              className='relative inline-flex items-center justify-center rounded-md p-2 text-gray-400 hover:bg-gray-700 hover:text-white focus:outline-none focus:ring-2 focus:ring-inset focus:ring-white'
              aria-controls='mobile-menu'
              aria-expanded={isMobileMenuOpen}
              onClick={() => setIsMobileMenuOpen(!isMobileMenuOpen)}
            >
              <span className='absolute -inset-0.5'></span>
              <span className='sr-only'>Open main menu</span>
              <svg
                className={`${isMobileMenuOpen ? 'hidden' : 'block'} h-6 w-6`}
                fill='none'
                viewBox='0 0 24 24'
                strokeWidth='1.5'
                stroke='currentColor'
                aria-hidden='true'
              >
                <path
                  strokeLinecap='round'
                  strokeLinejoin='round'
                  d='M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5'
                />
              </svg>
              <svg
                className={`${isMobileMenuOpen ? 'block' : 'hidden'} h-6 w-6`}
                fill='none'
                viewBox='0 0 24 24'
                strokeWidth='1.5'
                stroke='currentColor'
                aria-hidden='true'
              >
                <path
                  strokeLinecap='round'
                  strokeLinejoin='round'
                  d='M6 18L18 6M6 6l12 12'
                />
              </svg>
            </button>
          </div>

          {/* Logo and desktop menu */}
          <div className='flex flex-1 items-center justify-center sm:items-stretch sm:justify-start'>
            <Link className='flex flex-shrink-0 items-center' to='/'>
              <img className='h-10 w-auto' src={logo} alt='Igloo logo' />
              <span className='hidden sm:block text-white text-2xl font-bold ml-2'>
                Igloo
              </span>
            </Link>

            <div className='hidden sm:ml-6 sm:block'>
              <div className='flex space-x-2'>
                <NavLink to='/home' className='text-white hover:bg-gray-900 hover:text-white rounded-md px-3 py-2'>
                  Home
                </NavLink>
                <NavLink to='/movies' className='text-white hover:bg-gray-900 hover:text-white rounded-md px-3 py-2'>
                  Movies
                </NavLink>
                <NavLink to='/tv-shows' className='text-white hover:bg-gray-900 hover:text-white rounded-md px-3 py-2'>
                  TV Shows
                </NavLink>
                <NavLink to='/music' className='text-white hover:bg-gray-900 hover:text-white rounded-md px-3 py-2'>
                  Music
                </NavLink>
                <NavLink to='/photos' className='text-white hover:bg-gray-900 hover:text-white rounded-md px-3 py-2'>
                  Photos
                </NavLink>
              </div>
            </div>
          </div>

          {/* Right side menu */}
          <div className='absolute inset-y-0 right-0 flex items-center pr-2 sm:static sm:inset-auto sm:ml-6 sm:pr-0'>
            {!user ? (
              <Link to='/login' className='hidden sm:flex items-center text-white bg-gray-700 hover:bg-gray-900 hover:text-white rounded-md px-3 py-2'>
                <FaUser className='text-white mr-2' />
                <span>Login or Register</span>
              </Link>
            ) : (
              <div className='relative ml-3'>
                {/* Profile dropdown button */}
                <div>
                  <button
                    type='button'
                    className='relative flex rounded-full bg-gray-800 text-sm focus:outline-none focus:ring-2 focus:ring-white focus:ring-offset-2 focus:ring-offset-gray-800'
                    id='user-menu-button'
                    aria-expanded={isProfileMenuOpen}
                    aria-haspopup='true'
                    onClick={() => setIsProfileMenuOpen(prev => !prev)}
                  >
                    <span className='absolute -inset-1.5'></span>
                    <span className='sr-only'>Open user menu</span>
                    <img
                      className='h-8 w-8 rounded-full'
                      src={profileDefault}
                      alt=''
                      width={40}
                      height={40}
                    />
                  </button>
                </div>

                {/* Profile dropdown */}
                {isProfileMenuOpen && (
                  <div
                    id='user-menu'
                    className='absolute right-0 z-10 mt-2 w-48 origin-top-right rounded-md bg-white py-1 shadow-lg ring-1 ring-black ring-opacity-5 focus:outline-none'
                    role='menu'
                    aria-orientation='vertical'
                    aria-labelledby='user-menu-button'
                    tabIndex={-1}
                  >
                    <NavLink
                      to='/settings'
                      className='block px-4 py-2 text-sm text-gray-700'
                      role='menuitem'
                      tabIndex={-1}
                      id='user-menu-item-0'
                      onClick={() => {
                        setIsProfileMenuOpen(false);
                      }}
                    >
                      Settings
                    </NavLink>

                    <NavLink
                      to='/profile'
                      className='block px-4 py-2 text-sm text-gray-700'
                      role='menuitem'
                      tabIndex={-1}
                      id='user-menu-item-2'
                      onClick={() => {
                        setIsProfileMenuOpen(false);
                      }}
                    >
                      Profile
                    </NavLink>

                    <button
                      onClick={() => {
                        setIsProfileMenuOpen(false);
                        console.log("sign out");
                      }}
                      className='block px-4 py-2 text-sm text-gray-700'
                      role='menuitem'
                      tabIndex={-1}
                      id='user-menu-item-2'
                    >
                      Sign Out
                    </button>
                  </div>
                )}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Mobile menu */}
      <div className={`sm:hidden ${isMobileMenuOpen ? 'block' : 'hidden'}`} id='mobile-menu'>
        <div className='space-y-1 px-2 pb-3 pt-2'>
          <NavLink to='/home' className='text-white block rounded-md px-3 py-2 text-base font-medium' onClick={closeMobileMenu}>
            Home
          </NavLink>
          <NavLink to='/movies' className='text-white block rounded-md px-3 py-2 text-base font-medium' onClick={closeMobileMenu}>
            Movies
          </NavLink>
          <NavLink to='/tv-shows' className='text-white block rounded-md px-3 py-2 text-base font-medium' onClick={closeMobileMenu}>
            TV Shows
          </NavLink>
          <NavLink to='/music' className='text-white block rounded-md px-3 py-2 text-base font-medium' onClick={closeMobileMenu}>
            Music
          </NavLink>
          <NavLink to='/photos' className='text-white block rounded-md px-3 py-2 text-base font-medium' onClick={closeMobileMenu}>
            Photos
          </NavLink>
          {!user && (
            <Link to='/login' className='flex items-center text-white bg-gray-700 hover:bg-gray-900 hover:text-white rounded-md px-3 py-2' onClick={closeMobileMenu}>
              <FaUser className='text-white mr-2' />
              <span>Login or Register</span>
            </Link>
          )}
        </div>
      </div>
    </nav>
  );
}
